export default defineAppConfig({
  darkmode: true,
  themeLocation: 'theme.json',
  pages: [
    'pages/home/index',
    'pages/campus-guide/index',
    'pages/publish-entry/index',
    'pages/publish-post/index',
    'pages/publish-secondhand/index',
    'pages/student-takeout/index',
    'pages/student-secondhand/index',
    'pages/secondhand-chat/index',
    'pages/profile/index',
    'pages/blacklist/index',
    'pages/feedback/index',
    'pages/about/index',
    'pages/legal/index',
    'pages/merchant-messages/index',
    'pages/rider-messages/index',
    'pages/delivery-profile/index'
  ],
  subPackages: [
    // Keep public URLs unchanged. Ordinary subpackages retain access to shared
    // runtime/styles while leaving room for WeChat's own source compilation.
    {
      root: 'pages/merchant-publish',
      pages: ['index']
    },
    {
      root: 'pages/merchant-order-detail',
      pages: ['index']
    },
    {
      root: 'pages/merchant-wallet',
      pages: ['index']
    },
    {
      root: 'pages/rider-wallet',
      pages: ['index']
    },
    {
      root: 'pages/search',
      pages: ['index']
    },
    {
      root: 'pages/post-detail',
      pages: ['index']
    },
    {
      root: 'pages/secondhand-detail',
      pages: ['index']
    },
    {
      root: 'pages/user-profile',
      pages: ['index']
    },
    {
      root: 'pages/identity-application',
      pages: ['index']
    },
    {
      root: 'pages/my-content',
      pages: ['index']
    },
    {
      root: 'pages/edit-profile',
      pages: ['index']
    },
    {
      root: 'pages/settings',
      pages: ['index']
    },
    {
      root: 'pages/notifications',
      pages: ['index']
    },
    {
      root: 'pages/merchant-home',
      pages: ['index']
    },
    {
      root: 'pages/delivery-rider',
      pages: ['index']
    },
    {
      root: 'pages/delivery-conversation',
      pages: ['index']
    }
  ],
  preloadRule: {
    'pages/home/index': {
      network: 'all',
      packages: ['pages/search', 'pages/post-detail', 'pages/secondhand-detail', 'pages/user-profile', 'pages/notifications']
    },
    'pages/student-takeout/index': {
      network: 'all',
      packages: ['pages/search']
    },
    'pages/student-secondhand/index': {
      network: 'all',
      packages: ['pages/secondhand-detail']
    },
    'pages/profile/index': {
      network: 'all',
      packages: ['pages/identity-application', 'pages/my-content', 'pages/edit-profile', 'pages/settings']
    },
    'pages/merchant-messages/index': {
      network: 'all',
      packages: ['pages/merchant-home', 'pages/delivery-conversation']
    },
    'pages/rider-messages/index': {
      network: 'all',
      packages: ['pages/delivery-rider', 'pages/delivery-conversation']
    }
  },
  window: {
    backgroundTextStyle: '@backgroundHint',
    backgroundColor: '@backgroundDefault',
    navigationBarBackgroundColor: '@navigationBackground',
    navigationBarTitleText: '湛科生活圈',
    navigationBarTextStyle: '@navigationText'
  },
  requiredPrivateInfos: []
})
