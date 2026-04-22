module.exports = {
  apps: [
    {
      name: 'bsky-bot',
      script: 'node',
      args: '.',
      cwd: '/etc/sewagedata-socialbot',
      env: {
        NODE_ENV: 'production',
      },
    },
  ],
};
