(() => {
  const markActive = () => {
    const path = window.location.pathname || "/";
    const currentLocale = path.startsWith("/ru/") || path === "/ru/" ? "ru" : "en";

    const selectorLinks = document.querySelectorAll(".md-select__link[href]");
    for (const link of selectorLinks) {
      const href = link.getAttribute("href") || "";
      const isRu = href === "/ru/" || href.startsWith("/ru/");
      const locale = isRu ? "ru" : "en";
      if (locale === currentLocale) link.setAttribute("aria-current", "true");
      else link.removeAttribute("aria-current");
    }
  };

  document.addEventListener("DOMContentLoaded", markActive);
})();

