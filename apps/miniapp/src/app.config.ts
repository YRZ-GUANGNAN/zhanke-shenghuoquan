export default defineAppConfig({
  pages: [
    'pages/home/index',
    'pages/food/index',
    'pages/food-detail/index',
    'pages/market/index',
    'pages/publish/index',
    'pages/profile/index',
    'pages/my-content/index',
    'pages/post-detail/index',
    'pages/product-detail/index',
    'pages/delivery-merchant/index',
    'pages/delivery-rider/index',
    'pages/delivery-wallet/index',
    'pages/delivery-conversation/index',
    'pages/conversations/index',
    'pages/chat/index',
    'pages/notifications/index',
    'pages/media-history/index'
  ],
  window: {
    backgroundTextStyle: 'dark',
    backgroundColor: '#f2f5f9',
    enablePullDownRefresh: true,
    navigationBarBackgroundColor: '#f2f5f9',
    navigationBarTitleText: '',
    navigationBarTextStyle: 'black',
    navigationStyle: 'custom'
  },
  requiredPrivateInfos: []
})
