// pm2 設定：常駐 CTranslate2 sidecar。
//
// pm2 1.0.0 (Go) 用 goja 執行這個檔案，沒有 require／__dirname／process，
// 因此路徑只能寫字面值，args 必須是字串陣列。
module.exports = {
  apps: [
    {
      name: 'trans-ct2',
      namespace: 'trans',
      script: '/Users/shuk/projects/tools/trans/scripts/ct2_server.sh',
      args: [],
      cwd: '/Users/shuk/projects/tools/trans',
      autorestart: true,
      // 模型是延遲載入的：第一個請求會花幾秒把 1.2 GB 讀進記憶體，
      // 太短的重啟間隔會讓它永遠載不完。
      restart_delay: 5000,
      env: {
        TRANSLATE_CT2_PORT: '8765',
      },
      out_file: '/Users/shuk/.config/trans/logs/ct2.out.log',
      error_file: '/Users/shuk/.config/trans/logs/ct2.err.log',
    },
  ],
};
