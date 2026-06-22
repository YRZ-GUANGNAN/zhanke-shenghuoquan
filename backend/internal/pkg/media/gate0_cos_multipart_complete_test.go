package media

import cos "github.com/tencentyun/cos-go-sdk-v5"

func gate0V11CompleteOptions() *cos.CompleteMultipartUploadOptions {
	return &cos.CompleteMultipartUploadOptions{Parts: []cos.Object{{PartNumber: 1, ETag: `"gate0-etag"`}}}
}
