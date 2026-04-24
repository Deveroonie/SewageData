module.exports = {
  apps: [
    {
      name: "image-renderer",
      script: "/etc/sewagedata-image-renderer/server/index.js",
      cwd: "/etc/sewagedata-image-renderer",
      env: {
        PLAYWRIGHT_BROWSERS_PATH: "/etc/sewagedata-image-renderer/.playwright"
      }
    }
  ]
}
