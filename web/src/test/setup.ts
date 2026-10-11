import { cleanup } from "@testing-library/react";
import { afterEach, vi } from "vitest";

// o jsdom não implementa <dialog>.showModal/close
HTMLDialogElement.prototype.showModal = function () {
  this.setAttribute("open", "");
};
HTMLDialogElement.prototype.close = function () {
  this.removeAttribute("open");
  this.dispatchEvent(new Event("close"));
};

afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
});
