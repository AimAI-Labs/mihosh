module.exports = {
  apps: [
    {
      name: 'mihosh-docs-dev',
      script: './node_modules/.bin/next',
      args: 'dev',
      interpreter: 'node',
      watch: false,
      env: {
        NODE_ENV: 'development',
      },
    },
  ],
};
