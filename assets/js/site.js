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

/* ------------------------------------------------------ version menu ----- */

/* <details> handles opening and keyboard access on its own. All that is
   missing is the behaviour people expect from a menu: it should close when
   they click elsewhere or press Escape. */
function initVersionMenu() {
  const menu = document.querySelector('.version-menu');
  if (!menu) return;

  document.addEventListener('click', (event) => {
    if (menu.open && !menu.contains(event.target)) menu.open = false;
  });

  document.addEventListener('keydown', (event) => {
    if (event.key === 'Escape' && menu.open) {
      menu.open = false;
      menu.querySelector('summary')?.focus();
    }
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

    const language = languageOf(block);
    if (language) {
      const chip = document.createElement('span');
      chip.className = 'code-lang';
      chip.textContent = language;
      wrapper.append(chip);
    }

    wrapper.append(button);
  }
}

/* Rouge wraps a fenced block in `language-<name> highlighter-rouge`. */
function languageOf(block) {
  const holder = block.closest('[class*="language-"]') ?? block;
  const match = /language-([a-z0-9+#-]+)/i.exec(holder.className ?? '');
  if (!match) return null;
  const name = match[1].toLowerCase();
  return name === 'plaintext' || name === 'text' ? null : name;
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
   arrives as a plain code block. It arrives in one of two shapes, and both have
   to be handled:

     div.language-mermaid > div.highlight > pre > code   (a normal page)
     pre > code.language-mermaid                          (inside markdown="1")

   Kramdown drops the Rouge wrapper inside an HTML block carrying
   markdown="1", which is how the landing page is written. Replacing the code
   element alone left the rendered diagram nested inside the <pre>, framed like
   a code listing and inheriting its colours. */
function outermostBlock(node) {
  let element = node;
  while (element.parentElement) {
    const parent = element.parentElement;
    const isWrapper =
      parent.tagName === 'PRE'
      || parent.matches('div.highlight, pre.highlight, .code-block')
      || /\blanguage-/.test(parent.getAttribute('class') || '');
    if (!isWrapper) break;
    element = parent;
  }
  return element;
}

function collectDiagrams(language) {
  const content = document.getElementById('content');
  if (!content) return [];

  const found = [];
  const selector = [
    `code[class*="language-${language}"]`,
    `div[class*="language-${language}"]`,
    `pre[class*="language-${language}"]`,
  ].join(', ');

  for (const node of content.querySelectorAll(selector)) {
    if (!node.isConnected || node.closest('.diagram')) continue;

    const code = node.tagName === 'CODE' ? node : (node.querySelector('code') ?? node);
    const source = code.innerText.replace(/\n$/, '');
    if (!source.trim()) continue;

    const host = document.createElement('figure');
    host.className = `diagram diagram--${language}`;
    host.dataset.state = 'loading';
    host.textContent = 'Rendering diagram…';

    outermostBlock(code).replaceWith(host);
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

  /* Mermaid's palette is derived from the stylesheet's own tokens, so a colour
     change in main.css reaches the diagrams. The fallbacks follow the active
     theme: falling back to light values on a dark page produced diagrams with
     light fills and light text, which is unreadable. */
  const FALLBACK = {
    light: {
      surface: '#ffffff', raise: '#f1f5f9', bgSubtle: '#f8fafc',
      border: '#e4e9f0', borderStrong: '#cbd5e1',
      text: '#0f172a', muted: '#4a5568', faint: '#8b97a8',
      accentSoft: '#eef2ff', accentBorder: '#c7d2fe',
    },
    dark: {
      surface: '#101319', raise: '#161a22', bgSubtle: '#0c0e13',
      border: '#1e2430', borderStrong: '#2d3543',
      text: '#e9edf4', muted: '#9aa6b8', faint: '#66738a',
      accentSoft: '#171a2e', accentBorder: '#333c6b',
    },
  };

  const themeVariables = (theme) => {
    const css = getComputedStyle(document.documentElement);
    const fallback = FALLBACK[theme] ?? FALLBACK.light;
    const token = (name, key) => css.getPropertyValue(name).trim() || fallback[key];

    const surface = token('--surface', 'surface');
    const raise = token('--surface-2', 'raise');
    const bgSubtle = token('--bg-alt', 'bgSubtle');
    const border = token('--border', 'border');
    const borderStrong = token('--border-strong', 'borderStrong');
    const text = token('--text', 'text');
    const muted = token('--text-muted', 'muted');
    const faint = token('--text-faint', 'faint');
    const accentSoft = token('--accent-soft', 'accentSoft');
    const accentBorder = token('--accent-border', 'accentBorder');

    return {
      background: surface,
      mainBkg: raise,
      nodeBorder: borderStrong,
      nodeTextColor: text,
      primaryColor: raise,
      primaryTextColor: text,
      primaryBorderColor: borderStrong,
      secondaryColor: accentSoft,
      secondaryBorderColor: accentBorder,
      secondaryTextColor: text,
      tertiaryColor: bgSubtle,
      tertiaryBorderColor: border,
      tertiaryTextColor: muted,
      lineColor: faint,
      textColor: text,
      titleColor: muted,
      edgeLabelBackground: surface,
      clusterBkg: bgSubtle,
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
      themeVariables: { darkMode: theme === 'dark', ...themeVariables(theme) },
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

/* -------------------------------------------------------------- search --- */

/* A command palette over a small index built at publish time: every page, and
   every package, exported symbol and command in the reference. The index is
   fetched on first open, so it costs nothing to visitors who never search. */
function initSearch() {
  const dialog = document.getElementById('search-dialog');
  const input = document.getElementById('search-input');
  const results = document.getElementById('search-results');
  const hint = document.getElementById('search-hint');
  const trigger = document.getElementById('search-trigger');
  const close = document.getElementById('search-close');
  if (!dialog || !input || !results || !trigger) return;

  let index = null;
  let loading = null;
  let active = -1;

  const load = () => {
    if (index) return Promise.resolve(index);
    if (!loading) {
      loading = fetch(config.searchIndex || 'search.json')
        .then((response) => (response.ok ? response.json() : []))
        .then((data) => { index = data; return index; })
        .catch(() => { index = []; return index; });
    }
    return loading;
  };

  const open = () => {
    load();
    if (!dialog.open) dialog.showModal();
    input.value = '';
    render([]);
    input.focus();
  };

  const shut = () => { if (dialog.open) dialog.close(); };

  /* Rank on where the query lands: a title that starts with it beats one that
     merely contains it, and both beat a match in the surrounding context. */
  const score = (entry, query) => {
    const title = entry.title.toLowerCase();
    const context = (entry.context || '').toLowerCase();
    const text = (entry.text || '').toLowerCase();

    if (title === query) return 0;
    if (title.startsWith(query)) return 1;

    const word = title.split(/[\s./_-]+/).some((part) => part.startsWith(query));
    if (word) return 2;
    if (title.includes(query)) return 3;
    if (context.includes(query)) return 5;
    if (text.includes(query)) return 6;
    return Infinity;
  };

  const highlight = (value, query) => {
    const at = value.toLowerCase().indexOf(query);
    if (at < 0) return escapeHtml(value);
    return escapeHtml(value.slice(0, at))
      + '<mark>' + escapeHtml(value.slice(at, at + query.length)) + '</mark>'
      + escapeHtml(value.slice(at + query.length));
  };

  const render = (matches, query = '') => {
    results.innerHTML = '';
    active = matches.length ? 0 : -1;

    if (!query) {
      hint.hidden = false;
      hint.textContent = 'Type to search pages, packages, types and commands.';
      return;
    }
    if (!matches.length) {
      hint.hidden = false;
      hint.textContent = `Nothing matches “${query}”.`;
      return;
    }
    hint.hidden = true;

    for (const [position, entry] of matches.entries()) {
      const item = document.createElement('li');
      const link = document.createElement('a');
      link.className = 'search-result' + (position === 0 ? ' is-active' : '');
      link.href = entry.url;
      link.setAttribute('role', 'option');
      link.innerHTML =
        `<span class="search-kind">${escapeHtml(entry.kind)}</span>`
        + '<span class="search-result-text">'
        + `<span class="search-result-title">${highlight(entry.title, query)}</span>`
        + `<span class="search-result-context">${escapeHtml(entry.context || '')}</span>`
        + '</span>';
      item.append(link);
      results.append(item);
    }
  };

  const move = (delta) => {
    const links = [...results.querySelectorAll('.search-result')];
    if (!links.length) return;
    links[active]?.classList.remove('is-active');
    active = (active + delta + links.length) % links.length;
    links[active].classList.add('is-active');
    links[active].scrollIntoView({ block: 'nearest' });
  };

  input.addEventListener('input', async () => {
    const query = input.value.trim().toLowerCase();
    if (!query) return render([]);

    const entries = await load();
    const matches = entries
      .map((entry) => ({ entry, rank: score(entry, query) }))
      .filter((scored) => scored.rank !== Infinity)
      .sort((a, b) => a.rank - b.rank || a.entry.title.length - b.entry.title.length)
      .slice(0, 40)
      .map((scored) => scored.entry);

    render(matches, query);
  });

  input.addEventListener('keydown', (event) => {
    if (event.key === 'ArrowDown') { event.preventDefault(); move(1); }
    else if (event.key === 'ArrowUp') { event.preventDefault(); move(-1); }
    else if (event.key === 'Enter') {
      const link = results.querySelectorAll('.search-result')[active];
      if (link) { event.preventDefault(); window.location.href = link.href; }
    }
  });

  trigger.addEventListener('click', open);
  close?.addEventListener('click', shut);
  dialog.addEventListener('click', (event) => { if (event.target === dialog) shut(); });

  document.addEventListener('keydown', (event) => {
    if ((event.metaKey || event.ctrlKey) && event.key.toLowerCase() === 'k') {
      event.preventDefault();
      dialog.open ? shut() : open();
    } else if (event.key === '/' && !dialog.open && !isTyping(event.target)) {
      event.preventDefault();
      open();
    }
  });
}

function isTyping(target) {
  return target instanceof HTMLElement
    && (target.isContentEditable || ['INPUT', 'TEXTAREA', 'SELECT'].includes(target.tagName));
}

function escapeHtml(value) {
  return String(value).replace(/[&<>"']/g, (character) => ({
    '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;',
  }[character]));
}

/* ----------------------------------------------------------------- boot -- */

initTheme();
initNav();
initSearch();
initVersionMenu();
initPlantUML();
initMermaid();
initHeadingAnchors();
initTableScroll();
initCopyButtons();
initToc();
