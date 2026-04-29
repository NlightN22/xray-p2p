(() => {
  const getTheme = () =>
    document.body.getAttribute("data-md-color-scheme") === "slate"
      ? "dark"
      : "default";

  const init = () => {
    if (typeof mermaid === "undefined") return;
    mermaid.initialize({ startOnLoad: true, theme: getTheme() });
  };

  document.addEventListener("DOMContentLoaded", () => {
    init();

    const observer = new MutationObserver(() => {
      if (typeof mermaid === "undefined") return;
      mermaid.initialize({ startOnLoad: true, theme: getTheme() });
      mermaid.init(undefined, document.querySelectorAll(".mermaid"));
    });

    observer.observe(document.body, {
      attributes: true,
      attributeFilter: ["data-md-color-scheme"],
    });
  });
})();
