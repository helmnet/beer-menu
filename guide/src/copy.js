// Copy button for the user guide (contract: specs/018-guide-compose-install/contracts/guide-site.md).
// Wraps each <pre class="copy"> in a <div class="copy-block"> and adds a
// <button type="button" class="copy-button">Copy</button> to the wrapper, not to the pre, which
// scrolls sideways. CSS puts the button in the top-right corner of the box.
// A click writes that block's <code> text to the clipboard: the label shows "Copied", then "Copy"
// again after 2 seconds. If the clipboard is blocked or missing (plain HTTP, denied permission),
// the label shows "Copy failed" and the code text is selected so the user can copy it by hand.
// A classic script, not a module, so it also runs when the guide is opened from the file system.
(() => {
  for (const pre of document.querySelectorAll("pre.copy")) {
    const code = pre.querySelector("code");
    const button = document.createElement("button");
    button.type = "button";
    button.className = "copy-button";
    button.textContent = "Copy";
    // Returns the promise so the test can wait for it; a missing navigator.clipboard rejects too.
    button.addEventListener("click", () =>
      Promise.resolve()
        .then(() => navigator.clipboard.writeText(code.textContent))
        .then(
          () => {
            button.textContent = "Copied";
            setTimeout(() => (button.textContent = "Copy"), 2000);
          },
          () => {
            button.textContent = "Copy failed";
            getSelection().selectAllChildren(code);
          },
        ),
    );
    const wrapper = document.createElement("div");
    wrapper.className = "copy-block";
    pre.before(wrapper);
    wrapper.append(pre, button);
  }
})();
