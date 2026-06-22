package media

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

type uploadRepositoryStub struct {
	bundle         UploadBundle
	created        bool
	authorizeCalls int
	cancelCalls    int
	expireCalls    int
}

func (r *uploadRepositoryStub) CreateUploadBundle(_ context.Context, params CreateUploadBundleParams) (UploadBundle, bool, error) {
	r.bundle = params.Bundle
	r.bundle.Asset.ID = 1
	r.bundle.Ticket.AssetID = 1
	return r.bundle, r.created, nil
}
func (r *uploadRepositoryStub) FindUploadBundle(context.Context, string, uint64) (UploadBundle, error) {
	return r.bundle, nil
}
func (r *uploadRepositoryStub) AuthorizeUploadTicket(_ context.Context, _ string, _ uint64, _ uint64, _ time.Time) (UploadBundle, error) {
	r.authorizeCalls++
	r.bundle.Ticket.Status = TicketAuthorizationIssued
	r.bundle.Ticket.CredentialIssueCount = 1
	r.bundle.Ticket.RowVersion = 2
	return r.bundle, nil
}
func (r *uploadRepositoryStub) MarkUploadTicketUploaded(context.Context, string, uint64, uint64, time.Time) (UploadBundle, error) {
	return r.bundle, nil
}
func (r *uploadRepositoryStub) ConsumeUploadTicket(context.Context, string, uint64, uint64, time.Time) (UploadBundle, error) {
	return r.bundle, nil
}
func (r *uploadRepositoryStub) CancelUploadTicket(context.Context, string, uint64, uint64, time.Time) (UploadBundle, error) {
	r.cancelCalls++
	r.bundle.Ticket.Status = TicketCancelled
	return r.bundle, nil
}
func (r *uploadRepositoryStub) ExpireUploadTickets(context.Context, time.Time, int) (int, error) {
	r.expireCalls++
	return 1, nil
}

type uploadAuthorizerStub struct {
	request AuthorizationRequest
	err     error
}

func (a *uploadAuthorizerStub) IssueSinglePUT(_ context.Context, request AuthorizationRequest) (UploadAuthorization, error) {
	a.request = request
	if a.err != nil {
		return UploadAuthorization{}, a.err
	}
	return UploadAuthorization{
		UploadURL: "https://bucket.cos.ap-guangzhou.myqcloud.com/exact",
		RequiredHeaders: RequiredUploadHeaders{
			ContentType: request.ContentType, ContentMD5: request.MD5,
			ContentLength: request.Size, ForbidOverwrite: "true",
		},
		ExpiresAt: time.Now().Add(request.TTL),
	}, nil
}

func TestCreateTicketBuildsOpaqueUniqueStagingKey(t *testing.T) {
	now := time.Date(2026, 6, 21, 12, 0, 0, 0, time.UTC)
	repository := &uploadRepositoryStub{created: true}
	authorizer := &uploadAuthorizerStub{}
	service, err := NewUploadTicketService(repository, authorizer, UploadTicketServiceConfig{Environment: "test", Now: func() time.Time { return now }})
	if err != nil {
		t.Fatal(err)
	}
	input := validCreateUploadInput()
	bundle, created, err := service.CreateTicket(context.Background(), input)
	if err != nil || !created {
		t.Fatalf("created=%v err=%v", created, err)
	}
	if bundle.Asset.StagingKey == nil || !strings.HasPrefix(*bundle.Asset.StagingKey, "staging/test/") || strings.Contains(*bundle.Asset.StagingKey, "final/") {
		t.Fatalf("unsafe staging key: %v", bundle.Asset.StagingKey)
	}
	if bundle.Ticket.AssetID != bundle.Asset.ID || bundle.Reservation.TicketID != bundle.Ticket.TicketID {
		t.Fatal("ticket, asset, and reservation are not bound")
	}
}

func TestIssueAuthorizationNeverReturnsTemporarySecrets(t *testing.T) {
	now := time.Date(2026, 6, 21, 12, 0, 0, 0, time.UTC)
	repository := &uploadRepositoryStub{created: true}
	authorizer := &uploadAuthorizerStub{}
	service, _ := NewUploadTicketService(repository, authorizer, UploadTicketServiceConfig{Environment: "test", Now: func() time.Time { return now }})
	bundle, _, err := service.CreateTicket(context.Background(), validCreateUploadInput())
	if err != nil {
		t.Fatal(err)
	}
	authorization, err := service.IssueAuthorization(context.Background(), bundle.Asset.OwnerID, bundle.Ticket.TicketID, 1)
	if err != nil {
		t.Fatal(err)
	}
	if authorization.UploadURL == "" || authorization.RequiredHeaders.ForbidOverwrite != "true" || authorization.RequiredHeaders.ContentLength != bundle.Ticket.ExpectedSize {
		t.Fatalf("incomplete authorization: %#v", authorization)
	}
	if authorizer.request.ObjectKey != *bundle.Asset.StagingKey || authorizer.request.TTL > 2*time.Minute {
		t.Fatalf("authorizer received unsafe request: %#v", authorizer.request)
	}
}

func TestAuthorizerFailureCancelsOneTimeTicket(t *testing.T) {
	now := time.Date(2026, 6, 21, 12, 0, 0, 0, time.UTC)
	repository := &uploadRepositoryStub{created: true}
	authorizer := &uploadAuthorizerStub{err: errors.New("sts unavailable")}
	service, _ := NewUploadTicketService(repository, authorizer, UploadTicketServiceConfig{Environment: "test", Now: func() time.Time { return now }})
	bundle, _, _ := service.CreateTicket(context.Background(), validCreateUploadInput())
	if _, err := service.IssueAuthorization(context.Background(), bundle.Asset.OwnerID, bundle.Ticket.TicketID, 1); err == nil {
		t.Fatal("expected STS failure")
	}
	if repository.cancelCalls != 1 || repository.bundle.Ticket.Status != TicketCancelled {
		t.Fatal("failed authorization did not fail closed")
	}
}

func TestUploadDeclarationValidation(t *testing.T) {
	input := validCreateUploadInput()
	input.ExpectedContentType = "image/svg+xml"
	if err := validateUploadDeclaration(input); err != ErrInvalidUploadDeclaration {
		t.Fatalf("expected MIME rejection, got %v", err)
	}
	input = validCreateUploadInput()
	input.ExpectedMD5 = "forged"
	if err := validateUploadDeclaration(input); err != ErrInvalidUploadDeclaration {
		t.Fatalf("expected MD5 rejection, got %v", err)
	}
	input = validCreateUploadInput()
	input.Purpose, input.ExpectedSize = PurposeAvatar, 5<<20+1
	if err := validateUploadDeclaration(input); err != ErrInvalidUploadDeclaration {
		t.Fatalf("expected avatar size rejection, got %v", err)
	}
}

func validCreateUploadInput() CreateUploadTicketInput {
	return CreateUploadTicketInput{
		OwnerID: 42, Purpose: PurposePost, ExpectedSize: 1024,
		ExpectedMD5: "1B2M2Y8AsgTpgAmY7PhCfg==", ExpectedContentType: "image/webp",
		IdempotencyKey: "module3-unit-test",
	}
}
