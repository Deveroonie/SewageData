module.exports = {
  apps: [
    {
      name: 'bsky-bot',
      script: 'npm',
      args: 'start',
      cwd: '/etc/sewagedata-socialbot',
      env: {
        NODE_ENV: 'production',
      },
    },
  ],
};
