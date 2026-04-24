export default {
  apps: [
    {
      name: "image-renderer",
      script: "server/index.js",
      cwd: "/etc/sewagedata-image-renderer",
      env: {
        PLAYWRIGHT_BROWSERS_PATH: "/etc/sewagedata-image-renderer/.playwright"
      }
    }
  ]
}
