// SPDX-FileCopyrightText: Copyright (c) 2026 quarshell
// SPDX-License-Identifier: Apache-2.0

(function () {
  var flow = document.querySelector(".ws-flow");
  if (!flow) return;

  var tabs = Array.prototype.slice.call(flow.querySelectorAll("[data-flow-tab]"));
  var panels = Array.prototype.slice.call(flow.querySelectorAll("[data-flow-panel]"));
  var progress = Array.prototype.slice.call(flow.querySelectorAll(".ws-flow__progress span"));
  var reduceMotion = window.matchMedia("(prefers-reduced-motion: reduce)");
  var timers = [];
  var manual = false;
  var visible = false;
  var hero = flow.closest(".ws-hero");
  var mascot = hero && hero.querySelector("[data-mascot-stage]");
  var mascotPeekTimer = null;
  var mascotHideTimer = null;
  var mascotVisible = false;

  function stopMascotPeek() {
    window.clearTimeout(mascotPeekTimer);
    window.clearTimeout(mascotHideTimer);
    mascotPeekTimer = null;
    mascotHideTimer = null;
    if (mascot) mascot.classList.remove("is-peeking");
  }

  function scheduleMascotPeek() {
    if (!mascot || !mascotVisible || reduceMotion.matches || document.hidden) return;
    if (mascotPeekTimer || mascotHideTimer) return;
    mascotPeekTimer = window.setTimeout(function () {
      mascot.classList.add("is-peeking");
      mascotHideTimer = window.setTimeout(function () {
        mascot.classList.remove("is-peeking");
        mascotPeekTimer = null;
        mascotHideTimer = null;
        scheduleMascotPeek();
      }, 1450);
    }, 4200 + Math.random() * 4200);
  }

  function setMascotVisible(isVisible) {
    mascotVisible = isVisible;
    if (isVisible) scheduleMascotPeek();
    else stopMascotPeek();
  }

  function clearTimers() {
    timers.forEach(window.clearTimeout);
    timers = [];
  }

  function activate(index, animate, source) {
    clearTimers();
    var committed = false;
    var deferred = false;
    function commit() {
      if (committed) return;
      committed = true;
      flow.classList.toggle("ws-flow--playing", animate);
      tabs.forEach(function (tab, tabIndex) {
        var selected = tabIndex === index;
        tab.setAttribute("aria-selected", String(selected));
        tab.tabIndex = selected ? 0 : -1;
      });
      panels.forEach(function (panel, panelIndex) {
        panel.hidden = panelIndex !== index;
        panel.querySelectorAll(".ws-flow__line").forEach(function (line) {
          line.classList.remove("is-visible");
        });
      });
      progress.forEach(function (bar, barIndex) {
        bar.classList.toggle("is-complete", barIndex <= index);
      });

      flow.dispatchEvent(new CustomEvent("ws-flow:change", {
        detail: { index: index, source: source || "manual" },
      }));

      if (!animate) return;
      var lines = panels[index].querySelectorAll(".ws-flow__line");
      lines.forEach(function (line, lineIndex) {
        timers.push(window.setTimeout(function () {
          line.classList.add("is-visible");
        }, 180 + lineIndex * 620));
      });
      var nextIndex = (index + 1) % panels.length;
      timers.push(window.setTimeout(function () {
        if (!visible || manual || reduceMotion.matches) return;
        activate(nextIndex, true, "auto");
      }, 4900));
    }

    if (source === "manual" || source === "auto") {
      flow.dispatchEvent(new CustomEvent("ws-flow:beforechange", {
        detail: {
          index: index,
          source: source,
          defer: function () { deferred = true; return commit; },
        },
      }));
    }
    if (!deferred) commit();
  }

  tabs.forEach(function (tab, index) {
    tab.addEventListener("click", function () {
      manual = true;
      activate(index, false, "manual");
    });
    tab.addEventListener("keydown", function (event) {
      if (event.key !== "ArrowLeft" && event.key !== "ArrowRight") return;
      event.preventDefault();
      var next = (index + (event.key === "ArrowRight" ? 1 : tabs.length - 1)) % tabs.length;
      tabs[next].focus();
      tabs[next].click();
    });
  });

  if ("IntersectionObserver" in window) {
    new IntersectionObserver(function (entries) {
      visible = entries[0].isIntersecting;
      setMascotVisible(visible);
      if (visible && !manual && !reduceMotion.matches) activate(0, true, "initial");
      if (!visible) {
        clearTimers();
        flow.classList.remove("ws-flow--playing");
      }
    }, { threshold: 0.25 }).observe(flow);
  } else if (!reduceMotion.matches) {
    visible = true;
    setMascotVisible(true);
    activate(0, true, "initial");
  }

  if (hero && mascot) {
    hero.addEventListener("pointermove", function (event) {
      var bounds = hero.getBoundingClientRect();
      var x = Math.max(-1, Math.min(1, ((event.clientX - bounds.left) / bounds.width) * 2 - 1));
      var y = Math.max(-1, Math.min(1, 1 - ((event.clientY - bounds.top) / bounds.height) * 2));
      mascot.style.setProperty("--look-x", x.toFixed(3));
      mascot.style.setProperty("--look-y", y.toFixed(3));
    }, { passive: true });
    hero.addEventListener("pointerleave", function () {
      mascot.style.setProperty("--look-x", "0");
      mascot.style.setProperty("--look-y", "0");
    });
  }

  document.addEventListener("visibilitychange", function () {
    if (document.hidden) stopMascotPeek();
    else if (mascotVisible) scheduleMascotPeek();
  });

  if (typeof reduceMotion.addEventListener === "function") {
    reduceMotion.addEventListener("change", function () {
      if (reduceMotion.matches) stopMascotPeek();
      else if (mascotVisible) scheduleMascotPeek();
    });
  }
})();
