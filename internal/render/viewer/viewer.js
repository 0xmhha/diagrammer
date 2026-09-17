// The viewer does two things: move between levels, and answer "what is this
// one joined to" without the reader tracing lines with a finger.
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

  // Hovering a box fades everything it has nothing to do with.
  //
  // A page is a picture of the whole, and the question a reader arrives with is
  // usually about one part of it. Answering that by eye means following a line
  // across the page and hoping not to change lines at a crossing, which is the
  // work this saves.
  //
  // Faded rather than hidden, and nothing moves. A reader who has just found
  // the box they wanted should not have the page rearrange itself underneath
  // them, and what is faded is still there to be read.
  function inScene(scene, selector) {
    return Array.prototype.slice.call(scene.querySelectorAll(selector));
  }

  function linkBoxes(scene) {
    var boxes = inScene(scene, "[data-box-id]");
    var edges = inScene(scene, "[data-edge-id]");
    var labels = inScene(scene, "[data-edge-label-for]");
    var bars = inScene(scene, "[data-bar-box]");
    if (!boxes.length || !edges.length) return;

    var everything = boxes.concat(edges, labels, bars);

    function clear() {
      scene.classList.remove("focusing");
      everything.forEach(function (element) {
        element.classList.remove("related");
        element.classList.remove("origin");
      });
    }

    function focus(id) {
      // Object.create(null) rather than {}: a box id is any text the model
      // chose, and "toString", "constructor" and "__proto__" are all legal
      // ones. A plain object inherits those, so a box called toString would
      // have read as joined to everything.
      var near = Object.create(null);
      var lit = Object.create(null);
      near[id] = true;
      edges.forEach(function (edge) {
        var from = edge.getAttribute("data-edge-from");
        var to = edge.getAttribute("data-edge-to");
        if (from !== id && to !== id) return;
        lit[edge.getAttribute("data-edge-id")] = true;
        near[from] = true;
        near[to] = true;
      });

      clear();
      scene.classList.add("focusing");
      boxes.forEach(function (box) {
        var mine = box.getAttribute("data-box-id");
        if (!near[mine]) return;
        box.classList.add("related");
        if (mine === id) box.classList.add("origin");
      });
      edges.forEach(function (edge) {
        if (lit[edge.getAttribute("data-edge-id")]) edge.classList.add("related");
      });
      labels.forEach(function (label) {
        if (lit[label.getAttribute("data-edge-label-for")]) label.classList.add("related");
      });
      // A sequence diagram's executions belong to a lifeline, so they follow it.
      bars.forEach(function (bar) {
        if (near[bar.getAttribute("data-bar-box")]) bar.classList.add("related");
      });
    }

    boxes.forEach(function (box) {
      box.addEventListener("mouseenter", function () {
        focus(box.getAttribute("data-box-id"));
      });
      box.addEventListener("mouseleave", clear);
      // Keyboard and touch reach a box through focus rather than a pointer, and
      // the answer is the same either way.
      box.addEventListener("focusin", function () {
        focus(box.getAttribute("data-box-id"));
      });
      box.addEventListener("focusout", clear);
    });
    // Leaving the drawing altogether puts it back, which a box-by-box
    // mouseleave misses when the pointer jumps straight out of the page.
    scene.addEventListener("mouseleave", clear);
  }

  Array.prototype.slice.call(document.querySelectorAll("svg")).forEach(linkBoxes);

  var initial = window.location.hash.slice(1);
  show(panelFor(initial) ? initial : currentLevel());
})();
