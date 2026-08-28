/* Site behaviour: theme toggle, table of contents, code copy buttons, and
   diagram rendering. No build step — this is loaded as a module directly. */

const config = readConfig();

function readConfig() {
  const el = document.getElementById('site-config');
  if (!el) return {};
  try {
    return JSON.parse(el.textContent);
  } catch {
    return {};
  }
}

/* ---------------------------------------------------------------- theming */

const root = document.documentElement;
const themeListeners = new Set();

function currentTheme() {
  const explicit = root.getAttribute('data-theme');
  if (explicit === 'light' || explicit === 'dark') return explicit;
  return window.matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light';
}

function setTheme(theme) {
  root.setAttribute('data-theme', theme);
  try {
    localStorage.setItem('theme', theme);
  } catch { /* private mode: the choice just will not persist */ }
  themeListeners.forEach((fn) => fn(theme));
}

function initTheme() {
  const button = document.getElementById('theme-toggle');
  if (button) {
    button.addEventListener('click', () => {
      setTheme(currentTheme() === 'dark' ? 'light' : 'dark');
    });
  }

  // Follow the system while the visitor has not expressed a preference.
  window.matchMedia('(prefers-color-scheme: dark)').addEventListener('change', () => {
    if (!root.hasAttribute('data-theme')) {
      themeListeners.forEach((fn) => fn(currentTheme()));
    }
  });
}

/* ------------------------------------------------------------ mobile menu */

function initNav() {
  const toggle = document.getElementById('nav-toggle');
  const nav = document.getElementById('site-nav');
  if (!toggle || !nav) return;

  toggle.addEventListener('click', () => {
    const open = nav.classList.toggle('is-open');
    toggle.setAttribute('aria-expanded', String(open));
  });
}

/* ------------------------------------------------- table of contents ----- */

function initToc() {
  const content = document.getElementById('content');
  const list = document.getElementById('toc-list');
  if (!content || !list) return;

  const headings = [...content.querySelectorAll('h2[id], h3[id]')];
  if (headings.length < 2) {
    document.getElementById('toc')?.remove();
    return;
  }

  for (const heading of headings) {
    const item = document.createElement('li');
    item.className = heading.tagName === 'H3' ? 'toc-h3' : 'toc-h2';

    const link = document.createElement('a');
    link.href = `#${heading.id}`;
    link.textContent = heading.textContent.replace(/¶|#$/g, '').trim();
    item.append(link);
    list.append(item);
  }

  // Highlight the heading nearest the top of the viewport.
  const links = new Map(
    [...list.querySelectorAll('a')].map((a) => [a.hash.slice(1), a]),
  );
  let visible = new Set();

  const observer = new IntersectionObserver((entries) => {
    for (const entry of entries) {
      if (entry.isIntersecting) visible.add(entry.target.id);
      else visible.delete(entry.target.id);
    }

    const first = headings.find((h) => visible.has(h.id));
    links.forEach((a) => a.classList.remove('is-current'));
    if (first) links.get(first.id)?.classList.add('is-current');
  }, { rootMargin: '-80px 0px -70% 0px', threshold: 0 });

  headings.forEach((h) => observer.observe(h));
}

/* --------------------------------------------------- heading anchors ----- */

function initHeadingAnchors() {
  const content = document.getElementById('content');
  if (!content) return;

  for (const heading of content.querySelectorAll('h2[id], h3[id]')) {
    const anchor = document.createElement('a');
    anchor.className = 'heading-anchor';
    anchor.href = `#${heading.id}`;
    anchor.textContent = '#';
    anchor.setAttribute('aria-label', `Link to ${heading.textContent.trim()}`);
    heading.append(anchor);
  }
}

/* ------------------------------------------------------ copy buttons ----- */

function initCopyButtons() {
  const content = document.getElementById('content');
  if (!content || !navigator.clipboard) return;

  for (const block of content.querySelectorAll('div.highlight, pre.highlight')) {
    if (block.closest('.code-block') || block.closest('.diagram')) continue;
    if (block.closest('[class*="language-mermaid"], [class*="language-plantuml"]')) continue;

    const wrapper = document.createElement('div');
    wrapper.className = 'code-block';
    block.parentNode.insertBefore(wrapper, block);
    wrapper.append(block);

    const button = document.createElement('button');
    button.className = 'copy-button';
    button.type = 'button';
    button.textContent = 'Copy';

    button.addEventListener('click', async () => {
      const code = block.querySelector('code') ?? block;
      try {
        await navigator.clipboard.writeText(code.innerText.replace(/\n$/, ''));
        button.textContent = 'Copied';
        button.classList.add('is-copied');
        setTimeout(() => {
          button.textContent = 'Copy';
          button.classList.remove('is-copied');
        }, 1600);
      } catch {
        button.textContent = 'Press ⌘C';
      }
    });

    wrapper.append(button);
  }
}

/* ---------------------------------------------------------- responsive --- */

function initTableScroll() {
  const content = document.getElementById('content');
  if (!content) return;

  for (const table of content.querySelectorAll('table')) {
    if (table.closest('.table-scroll')) continue;
    const scroller = document.createElement('div');
    scroller.className = 'table-scroll';
    scroller.setAttribute('tabindex', '0');
    table.parentNode.insertBefore(scroller, table);
    scroller.append(table);
  }
}

/* ------------------------------------------------------------ diagrams --- */

/* Rouge has no lexer for `mermaid` or `plantuml`, so a fenced block of either
   arrives as a plain highlighted <pre>. Pull the source back out of it and
   hand it to a renderer. */
function collectDiagrams(language) {
  const content = document.getElementById('content');
  if (!content) return [];

  const found = [];
  const selector = `div.language-${language}, pre.language-${language}, code.language-${language}`;

  for (const node of content.querySelectorAll(selector)) {
    const block = node.closest('div.highlight, pre.highlight, div.language-' + language) ?? node;
    const code = block.querySelector('code') ?? block;
    const source = code.innerText.replace(/\n$/, '');
    if (!source.trim()) continue;

    const host = document.createElement('figure');
    host.className = `diagram diagram--${language}`;
    host.dataset.state = 'loading';
    host.textContent = 'Rendering diagram…';

    const outer = block.closest('.code-block') ?? block;
    outer.replaceWith(host);
    found.push({ host, source });
  }

  return found;
}

function showDiagramError(host, source, message) {
  host.dataset.state = 'error';
  host.textContent = message;
  const pre = document.createElement('pre');
  pre.textContent = source;
  host.append(pre);
}

async function initMermaid() {
  const diagrams = collectDiagrams('mermaid');
  if (diagrams.length === 0) return;

  const version = config.mermaidVersion || '11.12.0';
  let mermaid;
  try {
    ({ default: mermaid } = await import(
      `https://cdn.jsdelivr.net/npm/mermaid@${version}/dist/mermaid.esm.min.mjs`
    ));
  } catch {
    diagrams.forEach(({ host, source }) =>
      showDiagramError(host, source, 'Could not load Mermaid. The diagram source is below.'));
    return;
  }

  let sequence = 0;

  // Mermaid's palette is derived from the stylesheet's tokens rather than
  // duplicated here, so a colour change in main.css reaches the diagrams.
  const themeVariables = () => {
    const css = getComputedStyle(document.documentElement);
    const token = (name, fallback) => css.getPropertyValue(name).trim() || fallback;

    const surface = token('--surface', '#ffffff');
    const raise = token('--surface-raise', '#f2f4f7');
    const border = token('--border', '#e4e7ec');
    const borderStrong = token('--border-strong', '#d0d5dd');
    const text = token('--text', '#16191d');
    const muted = token('--text-muted', '#5b6472');
    const faint = token('--text-faint', '#8a94a3');
    const accentSoft = token('--accent-soft', '#eef2fe');
    const accentBorder = token('--accent-border', '#c3d0f8');

    return {
      background: surface,
      mainBkg: raise,
      nodeBorder: borderStrong,
      primaryColor: raise,
      primaryTextColor: text,
      primaryBorderColor: borderStrong,
      secondaryColor: accentSoft,
      secondaryBorderColor: accentBorder,
      secondaryTextColor: text,
      tertiaryColor: token('--bg-subtle', '#f7f8fa'),
      tertiaryBorderColor: border,
      tertiaryTextColor: muted,
      lineColor: faint,
      textColor: text,
      titleColor: muted,
      edgeLabelBackground: surface,
      clusterBkg: token('--bg-subtle', '#f7f8fa'),
      clusterBorder: border,
      // Sequence diagrams
      actorBkg: raise,
      actorBorder: borderStrong,
      actorTextColor: text,
      actorLineColor: faint,
      signalColor: text,
      signalTextColor: text,
      labelBoxBkgColor: raise,
      labelBoxBorderColor: borderStrong,
      labelTextColor: text,
      loopTextColor: muted,
      noteBkgColor: accentSoft,
      noteBorderColor: accentBorder,
      noteTextColor: text,
      activationBkgColor: accentSoft,
      activationBorderColor: accentBorder,
    };
  };

  const render = async (theme) => {
    mermaid.initialize({
      startOnLoad: false,
      securityLevel: 'strict',
      // 'base' plus explicit variables, rather than the stock light and dark
      // themes, so the diagrams match the page instead of merely coexisting.
      theme: 'base',
      fontFamily: getComputedStyle(document.body).fontFamily,
      themeVariables: { darkMode: theme === 'dark', ...themeVariables() },
    });

    for (const { host, source } of diagrams) {
      try {
        const { svg } = await mermaid.render(`mermaid-${sequence++}`, source);
        host.innerHTML = svg;
        host.dataset.state = 'ready';
      } catch (error) {
        showDiagramError(host, source, `Diagram failed to render: ${error?.message ?? error}`);
      }
    }
  };

  await render(currentTheme());
  // Mermaid bakes colours into the SVG, so a theme change means a re-render.
  themeListeners.add((theme) => { render(theme); });
}

/* PlantUML has no browser renderer, so the source goes to a server. That is
   off unless `plantuml_server` is set in _config.yml, because it means sending
   the diagram to a third party. `~h` selects the server's hex encoding, which
   avoids needing a deflate implementation here. */
function initPlantUML() {
  const diagrams = collectDiagrams('plantuml');
  if (diagrams.length === 0) return;

  const server = (config.plantumlServer || '').replace(/\/$/, '');
  if (!server) {
    diagrams.forEach(({ host, source }) => showDiagramError(
      host, source,
      'PlantUML rendering is off. Set `plantuml_server` in _config.yml to enable it.',
    ));
    return;
  }

  const encoder = new TextEncoder();
  for (const { host, source } of diagrams) {
    const hex = [...encoder.encode(source)]
      .map((b) => b.toString(16).padStart(2, '0'))
      .join('');

    const img = document.createElement('img');
    img.loading = 'lazy';
    img.alt = 'PlantUML diagram';
    img.src = `${server}/svg/~h${hex}`;
    img.addEventListener('error', () =>
      showDiagramError(host, source, 'The PlantUML server did not return a diagram.'));
    img.addEventListener('load', () => { host.dataset.state = 'ready'; });

    host.textContent = '';
    host.append(img);
  }
}

/* ----------------------------------------------------------------- boot -- */

initTheme();
initNav();
initPlantUML();
initMermaid();
initHeadingAnchors();
initTableScroll();
initCopyButtons();
initToc();
