/* ============================================================
   agent-platform · 前端联调台
   只依赖后端已实现的接口：注册 / 验证邮箱 / 登录 / 刷新 token / 忘记密码
   ============================================================ */
(() => {
  'use strict';

  const API_BASE = window.API_BASE || '';
  const STORE_KEY = 'ap_session';

  /* ---------- 错误码文案（对齐 common/biz/error.go） ---------- */
  const CODE_TEXT = {
    999: 'db error（数据库/发信失败，去看后端日志）',
    1001: '用户名已存在',
    1002: '邮箱已存在',
    1003: '密码格式错误',
    1004: '无效的 token',
    1005: '用户不存在',
    1006: '邮箱未验证',
    1007: 'token 生成失败',
    400: '参数不合法（binding 校验没过）',
    401: '未授权 / 令牌已过期',
    500: '服务端错误',
  };

  /* ---------- DOM helpers ---------- */
  const $ = (sel, root = document) => root.querySelector(sel);
  const $$ = (sel, root = document) => Array.from(root.querySelectorAll(sel));

  /* ---------- 会话状态 ---------- */
  let state = { token: '', refreshToken: '', expire: 0, refreshExpire: 0, userInfo: null };

  function loadSession() {
    try {
      const raw = localStorage.getItem(STORE_KEY);
      if (raw) state = Object.assign(state, JSON.parse(raw));
    } catch (e) { /* 损坏就丢弃 */ }
  }
  function saveSession() {
    localStorage.setItem(STORE_KEY, JSON.stringify(state));
  }
  function clearSession() {
    state = { token: '', refreshToken: '', expire: 0, refreshExpire: 0, userInfo: null };
    localStorage.removeItem(STORE_KEY);
  }

  /* ---------- 提示条 ---------- */
  function msg(el, text, ok) {
    const node = typeof el === 'string' ? $(el) : el;
    if (!node) return;
    node.className = 'msg is-on' + (ok ? ' is-ok' : '');
    node.innerHTML = '<span class="tag">' + (ok ? 'ok' : 'err') + '</span> ' + escapeHtml(text);
  }
  function clearMsg(el) {
    const node = typeof el === 'string' ? $(el) : el;
    if (node) { node.className = 'msg'; node.textContent = ''; }
  }
  function escapeHtml(s) {
    return String(s).replace(/[&<>"']/g, c => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[c]));
  }

  /* ---------- 请求日志（终端） ---------- */
  let logCount = 0;
  function logLine(method, path, status, ms, data, isBad) {
    const body = $('#reqlogBody');
    if (logCount === 0) body.innerHTML = '';
    logCount += 1;

    const line = document.createElement('p');
    line.className = 'rl';
    line.innerHTML =
      '<span class="rl-m">' + escapeHtml(method) + '</span>' +
      '<span class="rl-p">' + escapeHtml(path) + '</span>' +
      '<span class="rl-s ' + (isBad ? 'is-bad' : 'is-ok') + '">' + escapeHtml(String(status)) + '</span>' +
      '<span class="rl-t">' + Math.round(ms) + 'ms</span>';
    body.appendChild(line);

    if (data !== undefined) {
      const res = document.createElement('p');
      res.className = 'res';
      let text = typeof data === 'string' ? data : JSON.stringify(data);
      if (text && text.length > 600) text = text.slice(0, 600) + ' …(截断)';
      res.textContent = '  ' + text;
      body.appendChild(res);
    }
    body.scrollTop = body.scrollHeight;
  }

  /* ---------- 统一请求 ---------- */
  async function call(method, path, body, opts = {}) {
    const headers = { 'Content-Type': 'application/json' };
    const token = opts.token || state.token;
    if (opts.auth && token) headers['Authorization'] = 'Bearer ' + token;

    const t0 = performance.now();
    let res;
    try {
      res = await fetch(API_BASE + path, {
        method,
        headers,
        body: body ? JSON.stringify(body) : undefined,
      });
    } catch (e) {
      logLine(method, path, 'NET', performance.now() - t0, String(e), true);
      return { code: 'NET_ERR', msg: '请求发不出去：' + e.message + '（后端起了吗？serve.py 起了吗？）' };
    }

    const raw = await res.text();
    let data;
    try { data = JSON.parse(raw); } catch (e) { data = raw; }
    const ms = performance.now() - t0;

    if (res.status === 302 || res.redirected) {
      logLine(method, path, res.status, ms, data, false);
      return { code: 302, msg: '已重定向' };
    }
    logLine(method, path, res.status, ms, data, !res.ok || (data && data.code !== 200));
    if (!res.ok) return { code: res.status, msg: 'HTTP ' + res.status };
    return data;
  }

  function describe(r) {
    const code = r && r.code;
    const text = (r && r.msg) || '';
    return '[' + code + '] ' + (text || CODE_TEXT[code] || '未知错误');
  }

  /* ---------- 视图路由 ---------- */
  const PATHS = { login: '~/login', register: '~/register', forgot: '~/forgot', console: '~/console' };

  function show(view) {
    if (view === 'console' && !state.token) {
      msg('#loginMsg', '请先登录');
      view = 'login';
    }
    $$('.view').forEach(v => v.classList.toggle('is-active', v.id === 'view-' + view));
    $$('.gonav a').forEach(a => a.classList.toggle('is-active', a.dataset.view === view));
    $('#winbarPath').textContent = PATHS[view] || '~/';
    $('#gonav').classList.remove('is-open');
    if (view === 'console') renderConsole();
    window.scrollTo({ top: 0, behavior: 'auto' });
  }

  function currentView() {
    const h = (location.hash || '').replace(/^#\/?/, '');
    return PATHS[h] ? h : 'login';
  }

  /* ---------- 导航状态 ---------- */
  function renderNav() {
    const on = !!state.token;
    $('#navStatus').classList.toggle('is-on', on);
    $('#navStatusText').textContent = on
      ? (state.userInfo && state.userInfo.username ? state.userInfo.username : '已登录')
      : '未登录';
  }

  /* ---------- 登录 ---------- */
  $('#loginForm').addEventListener('submit', async (e) => {
    e.preventDefault();
    const f = e.target;
    clearMsg('#loginMsg');
    const r = await call('POST', '/api/v1/auth/login', {
      username: f.username.value.trim(),
      password: f.password.value,
    });
    if (r.code !== 200) return msg('#loginMsg', describe(r));
    state.token = r.data.token || '';
    state.refreshToken = r.data.refreshToken || '';
    state.expire = r.data.expire || 0;
    state.refreshExpire = r.data.refreshExpire || 0;
    state.userInfo = r.data.userInfo || null;
    saveSession();
    renderNav();
    msg('#consoleMsg', '登录成功', true);
    location.hash = '#/console';
    show('console');
  });

  /* ---------- 注册 ---------- */
  $('#registerForm').addEventListener('submit', async (e) => {
    e.preventDefault();
    const f = e.target;
    clearMsg('#registerMsg');
    const email = f.email.value.trim();
    const r = await call('POST', '/api/v1/auth/register', {
      username: f.username.value.trim(),
      password: f.password.value,
      email: email,
    });
    if (r.code !== 200) return msg('#registerMsg', describe(r));
    msg('#registerMsg', (r.data && r.data.message) + ' → 收件箱：' + email + '（点完链接回来登录）', true);
  });

  /* ---------- 忘记密码：三步 ---------- */
  let resetCtx = { email: '', token: '' };

  function gotoStep(n) {
    $$('.steps i').forEach(i => i.classList.toggle('is-on', Number(i.dataset.stepDot) === n));
    $$('.step').forEach(s => s.classList.toggle('is-on', Number(s.dataset.step) === n));
    clearMsg('#forgotMsg');
  }

  $('#step1').addEventListener('submit', async (e) => {
    e.preventDefault();
    const email = e.target.email.value.trim();
    const r = await call('POST', '/api/v1/auth/forgot-password', { email });
    if (r.code !== 200) return msg('#forgotMsg', describe(r));
    resetCtx.email = email;
    $('#step2').email.value = email;
    $('#step3').email.value = email;
    gotoStep(2);
    msg('#forgotMsg', (r.data && r.data.message) + '（5 分钟内有效）', true);
  });

  $('#step2').addEventListener('submit', async (e) => {
    e.preventDefault();
    const email = e.target.email.value.trim();
    const code = e.target.code.value.trim();
    const r = await call('POST', '/api/v1/auth/verify-code', { email, code });
    if (r.code !== 200) return msg('#forgotMsg', describe(r));
    resetCtx.email = email;
    resetCtx.token = (r.data && r.data.token) || '';
    $('#step3').email.value = email;
    $('#step3').token.value = resetCtx.token;
    gotoStep(3);
    msg('#forgotMsg', '验证码通过，已换到 resetToken，填新密码即可', true);
  });

  $('#step3').addEventListener('submit', async (e) => {
    e.preventDefault();
    const r = await call('POST', '/api/v1/auth/reset-password', {
      email: e.target.email.value.trim(),
      token: e.target.token.value.trim(),
      newPassword: e.target.newPassword.value,
    });
    if (r.code !== 200) return msg('#forgotMsg', describe(r));
    msg('#forgotMsg', (r.data && r.data.message) + '，可以去登录了', true);
    setTimeout(() => { location.hash = '#/login'; }, 900);
  });

  $$('[data-goto-step]').forEach(btn => {
    btn.addEventListener('click', () => gotoStep(Number(btn.dataset.gotoStep)));
  });

  /* ---------- 控制台 ---------- */
  function fmtTime(ms) {
    if (!ms) return '';
    const d = new Date(ms);
    const pad = n => String(n).padStart(2, '0');
    return d.getFullYear() + '-' + pad(d.getMonth() + 1) + '-' + pad(d.getDate()) +
      ' ' + pad(d.getHours()) + ':' + pad(d.getMinutes()) + ':' + pad(d.getSeconds());
  }
  function leftText(ms) {
    if (!ms) return '';
    const diff = ms - Date.now();
    if (diff <= 0) return '（已过期）';
    const h = Math.floor(diff / 3600000);
    const m = Math.floor((diff % 3600000) / 60000);
    return '（还剩 ' + h + 'h ' + m + 'm）';
  }

  function renderConsole() {
    const u = state.userInfo || {};
    const rows = [
      ['id', u.id],
      ['username', u.username],
      ['role', u.role],
      ['status', u.status === 1 ? '1 · normal' : u.status === 2 ? '2 · disable' : u.status === 3 ? '3 · pending' : u.status],
      ['email', u.email],
      ['avatar', u.avatar],
    ];
    $('#userInfoKv').innerHTML = rows.map(([k, v]) =>
      '<dt>' + escapeHtml(k) + '</dt>' +
      '<dd class="' + (v === '' || v === undefined || v === null ? 'empty' : '') + '">' +
      escapeHtml(v === '' || v === undefined || v === null ? '(空)' : String(v)) + '</dd>'
    ).join('');

    $('#tokenBox').innerHTML = '<b>token</b> ' + escapeHtml(state.token || '(空)') +
      '<br><b>expire</b> ' + escapeHtml(fmtTime(state.expire)) + ' ' + escapeHtml(leftText(state.expire));
    $('#refreshBox').innerHTML = '<b>refreshToken</b> ' + escapeHtml(state.refreshToken || '(空)') +
      '<br><b>refreshExpire</b> ' + escapeHtml(fmtTime(state.refreshExpire)) + ' ' + escapeHtml(leftText(state.refreshExpire));

    $('#sessionTerm').innerHTML =
      '<p><span class="p">$</span> whoami</p>' +
      '<p class="out">&gt; ' + escapeHtml(u.username || '(未登录)') + ' · ' + escapeHtml(u.role || '-') + '</p>' +
      '<p><span class="p">$</span> echo $TOKEN</p>' +
      '<p class="out">&gt; ' + escapeHtml(state.token ? state.token.slice(0, 24) + '…' : '(无)') + '</p>' +
      '<p><span class="p">$</span> <span class="cursor" aria-hidden="true"></span></p>';
  }

  $('#btnRefresh').addEventListener('click', async () => {
    clearMsg('#consoleMsg');
    if (!state.refreshToken) return msg('#consoleMsg', '没有 refreshToken，先登录');
    const r = await call('POST', '/api/v1/auth/refresh-token', { refreshToken: state.refreshToken });
    if (r.code !== 200) return msg('#consoleMsg', describe(r));
    state.token = r.data.token || '';
    state.refreshToken = r.data.refreshToken || '';
    state.expire = r.data.expire || 0;
    state.refreshExpire = r.data.refreshExpire || 0;
    if (r.data.userInfo) state.userInfo = r.data.userInfo;
    saveSession();
    renderConsole();
    msg('#consoleMsg', 'token 已刷新', true);
  });

  $('#btnSubscription').addEventListener('click', async () => {
    clearMsg('#consoleMsg');
    const r = await call('GET', '/api/v1/subscription/current', null, { auth: true });
    if (r.code !== 200) return msg('#consoleMsg', describe(r));
    msg('#consoleMsg', '订阅接口返回：plan=' + r.data.plan + ' / ' + r.data.duration +
      ' / 上限 agents=' + r.data.configs.maxAgents, true);
  });

  $('#btnLogout').addEventListener('click', () => {
    clearSession();
    renderNav();
    location.hash = '#/login';
    show('login');
    msg('#loginMsg', '已退出登录', true);
  });

  $('#btnClearLog').addEventListener('click', () => {
    logCount = 0;
    $('#reqlogBody').innerHTML = '<p class="dim">$ 等待请求…</p>';
  });

  /* ---------- 导航交互 ---------- */
  $$('.gonav a').forEach(a => a.addEventListener('click', () => {
    setTimeout(() => show(currentView()), 0);
  }));
  $('.gonav-toggle').addEventListener('click', (e) => {
    const nav = $('#gonav');
    const open = nav.classList.toggle('is-open');
    e.currentTarget.setAttribute('aria-expanded', String(open));
  });
  window.addEventListener('hashchange', () => show(currentView()));

  /* ---------- 启动 ---------- */
  loadSession();
  renderNav();
  $('#apiBaseLabel').textContent = (API_BASE || location.origin) + '/api';
  show(currentView() === 'console' ? 'console' : currentView());
  if (location.pathname === '/login' && !state.token) {
    msg('#loginMsg', '如果你刚点完邮件里的验证链接，现在直接登录就行', true);
  }
})();
