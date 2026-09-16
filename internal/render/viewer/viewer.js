// The viewer does one thing: move between levels.
//
// Everything it needs is in the page's own attributes, so it never fetches, and
// the page works from a file:// URL with nothing else present. The attributes it
// reads are the contract in internal/render/dom.go, and a generated test asserts
// this file reads nothing that is not in it.
(function () {
  "use strict";

  var panels = Array.prototype.slice.call(document.querySelectorAll("[data-level]"));
  var buttons = Array.prototype.slice.call(document.querySelectorAll("[data-level-link]"));
  var history = [];

  function panelFor(id) {
    for (var i = 0; i < panels.length; i++) {
      if (panels[i].getAttribute("data-level") === id) return panels[i];
    }
    return null;
  }

  function show(id) {
    var found = false;
    panels.forEach(function (panel) {
      var mine = panel.getAttribute("data-level") === id;
      // The panel and the drawing inside it share the attribute, so only the
      // outermost one is hidden; hiding both would hide the drawing twice and
      // leave the section taking up space.
      if (panel.tagName.toLowerCase() === "section") panel.hidden = !mine;
      if (mine) found = true;
    });
    if (!found) return;
    buttons.forEach(function (button) {
      button.setAttribute("aria-current", button.getAttribute("data-level-link") === id ? "true" : "false");
    });
    if (window.location.hash.slice(1) !== id) {
      window.history.replaceState(null, "", "#" + id);
    }
  }

  function go(id, remember) {
    if (!panelFor(id)) return;
    if (remember) history.push(currentLevel());
    show(id);
  }

  function currentLevel() {
    for (var i = 0; i < panels.length; i++) {
      if (panels[i].tagName.toLowerCase() === "section" && !panels[i].hidden) {
        return panels[i].getAttribute("data-level");
      }
    }
    return panels.length ? panels[0].getAttribute("data-level") : "";
  }

  buttons.forEach(function (button) {
    button.addEventListener("click", function () {
      history = [];
      go(button.getAttribute("data-level-link"), false);
    });
  });

  document.querySelectorAll("[data-opens]").forEach(function (box) {
    box.addEventListener("click", function () {
      go(box.getAttribute("data-opens"), true);
    });
  });

  document.querySelectorAll("[data-level-back]").forEach(function (back) {
    back.addEventListener("click", function () {
      var previous = history.pop();
      if (previous) show(previous);
    });
  });

  window.addEventListener("hashchange", function () {
    go(window.location.hash.slice(1), false);
  });

  var initial = window.location.hash.slice(1);
  show(panelFor(initial) ? initial : currentLevel());
})();
