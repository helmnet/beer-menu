// Starts read-only Swagger UI for this API's contract (feature 9, research R5). A file, not an
// inline script, so the page's CSP can stay script-src 'self'.
//
// Why each read-only option is set:
// - supportedSubmitMethods: [] removes "Try it out", so the page never sends an API request.
// - readOnly hides the Authorize and per-operation lock buttons, so nobody types a credential here.
// - queryConfigEnabled: false stops ?url= or ?configUrl= from loading another document.
// - validatorUrl: null stops the request to the online validator (no other origin).
// - showExtensions: true shows each operation's x-access.
// - BaseLayout has no top bar with a URL box.
const readOnly = () => ({
  wrapComponents: {
    authorizeBtn: () => () => null,
    authorizeOperationBtn: () => () => null,
  },
});

window.ui = SwaggerUIBundle({
  url: "/api/openapi.yaml",
  dom_id: "#swagger-ui",
  layout: "BaseLayout",
  presets: [SwaggerUIBundle.presets.apis],
  plugins: [readOnly],
  supportedSubmitMethods: [],
  showExtensions: true,
  queryConfigEnabled: false,
  validatorUrl: null,
  deepLinking: true,
  docExpansion: "list",
});
