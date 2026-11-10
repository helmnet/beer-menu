import { configure } from "@testing-library/react";

// The first render in a file can pass 1000 ms on a loaded machine (jsdom startup and imports).
configure({ asyncUtilTimeout: 5000 });

// jsdom has HTMLDialogElement but no showModal or close (research R3). These stubs only toggle
// the open attribute, so tests can find dialogs by role; real focus trapping and Escape are
// checked in the browser gate.
if (typeof HTMLDialogElement !== "undefined" && !HTMLDialogElement.prototype.showModal) {
  HTMLDialogElement.prototype.showModal = function (this: HTMLDialogElement) {
    this.open = true;
  };
  HTMLDialogElement.prototype.close = function (this: HTMLDialogElement) {
    this.open = false;
    this.dispatchEvent(new Event("close"));
  };
}
