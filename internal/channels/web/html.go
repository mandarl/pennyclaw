package web

const indexHTML = `<!DOCTYPE html>
<html lang="en" data-theme="dark">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>PennyClaw</title>
<!-- CDN: Markdown rendering -->
<script src="https://cdn.jsdelivr.net/npm/marked@15/marked.min.js"></script>
<!-- CDN: XSS sanitization -->
<script src="https://cdn.jsdelivr.net/npm/dompurify@3/dist/purify.min.js"></script>
<!-- CDN: Syntax highlighting -->
<link rel="stylesheet" href="https://cdn.jsdelivr.net/gh/highlightjs/cdn-release@11/build/styles/github-dark.min.css" id="hljs-theme-dark">
<link rel="stylesheet" href="https://cdn.jsdelivr.net/gh/highlightjs/cdn-release@11/build/styles/github.min.css" id="hljs-theme-light" disabled>
<script src="https://cdn.jsdelivr.net/gh/highlightjs/cdn-release@11/build/highlight.min.js"></script>
<!-- CDN: marked-highlight extension for syntax highlighting -->
<script src="https://cdn.jsdelivr.net/npm/marked-highlight@2/lib/index.umd.min.js"></script>
<style>
  :root { --bg: #0a0a0a; --bg2: #111; --bg3: #1a1a1a; --border: #222; --border2: #333; --text: #e0e0e0; --text2: #999; --text3: #666; --text4: #555; --accent: #f5a623; --accent-bg: rgba(245,166,35,0.1); --user-bg: #1a3a5c; --success: #4caf50; --error: #ef4444; --warn: #f5a623; --code-bg: #1a1a1a; --scrollbar: #333; }
  [data-theme="light"] { --bg: #f8f9fa; --bg2: #fff; --bg3: #f0f0f0; --border: #ddd; --border2: #ccc; --text: #1a1a1a; --text2: #555; --text3: #888; --text4: #aaa; --accent: #d4880f; --accent-bg: rgba(212,136,15,0.08); --user-bg: #d0e4f5; --success: #2e7d32; --error: #c62828; --warn: #e65100; --code-bg: #f5f5f5; --scrollbar: #ccc; }

  * { margin: 0; padding: 0; box-sizing: border-box; }
  body { font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif; background: var(--bg); color: var(--text); height: 100vh; display: flex; overflow: hidden; }

  /* Sidebar */
  .sidebar { width: 260px; background: var(--bg2); border-right: 1px solid var(--border); display: flex; flex-direction: column; flex-shrink: 0; transition: margin-left 0.25s ease; }
  .sidebar.collapsed { margin-left: -260px; }
  .sidebar-header { padding: 16px; border-bottom: 1px solid var(--border); display: flex; align-items: center; justify-content: space-between; }
  .sidebar-header .brand { font-size: 18px; font-weight: 700; color: var(--accent); }
  .sidebar-header button { background: none; border: none; color: var(--text2); cursor: pointer; font-size: 18px; padding: 4px; }
  .sidebar-header button:hover { color: var(--accent); }
  .new-chat-btn { margin: 12px 16px; padding: 10px; background: var(--accent); color: #000; border: none; border-radius: 8px; font-size: 13px; font-weight: 600; cursor: pointer; transition: opacity 0.2s; }
  .new-chat-btn:hover { opacity: 0.85; }
  .session-list { flex: 1; overflow-y: auto; padding: 8px; }
  .session-list::-webkit-scrollbar { width: 4px; }
  .session-list::-webkit-scrollbar-thumb { background: var(--scrollbar); border-radius: 2px; }
  .session-item { padding: 10px 12px; border-radius: 8px; cursor: pointer; font-size: 13px; color: var(--text2); margin-bottom: 2px; display: flex; align-items: center; justify-content: space-between; transition: background 0.15s; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
  .session-item:hover { background: var(--bg3); }
  .session-item.active { background: var(--accent-bg); color: var(--accent); }
  .session-item .del-btn { display: none; background: none; border: none; color: var(--error); cursor: pointer; font-size: 14px; padding: 2px 4px; flex-shrink: 0; }
  .session-item:hover .del-btn { display: block; }
  .session-item .preview-text { overflow: hidden; text-overflow: ellipsis; flex: 1; }
  .sidebar-footer { padding: 12px 16px; border-top: 1px solid var(--border); display: flex; flex-direction: column; gap: 4px; }
  .sidebar-footer button { background: none; border: none; color: var(--text2); cursor: pointer; font-size: 12px; padding: 6px 8px; border-radius: 6px; text-align: left; transition: all 0.15s; display: flex; align-items: center; gap: 8px; }
  .sidebar-footer button:hover { background: var(--bg3); color: var(--text); }
  .token-display { font-size: 11px; color: var(--text4); padding: 4px 8px; }

  /* Main area */
  .main { flex: 1; display: flex; flex-direction: column; min-width: 0; }

  /* Header */
  .header { padding: 12px 20px; background: var(--bg2); border-bottom: 1px solid var(--border); display: flex; align-items: center; gap: 12px; }
  .header .toggle-sidebar { background: none; border: none; color: var(--text2); cursor: pointer; font-size: 20px; padding: 2px 6px; }
  .header .toggle-sidebar:hover { color: var(--accent); }
  .header .title { font-size: 15px; font-weight: 600; color: var(--text); flex: 1; }
  .header .status { font-size: 12px; color: var(--success); display: flex; align-items: center; gap: 6px; }
  .header .status::before { content: ''; width: 7px; height: 7px; background: var(--success); border-radius: 50%; }
  .hdr-btn { background: none; border: 1px solid var(--border2); border-radius: 6px; padding: 4px 10px; color: var(--text2); font-size: 12px; cursor: pointer; transition: all 0.15s; }
  .hdr-btn:hover { border-color: var(--accent); color: var(--accent); }
  .hdr-btn.active { border-color: var(--accent); color: var(--accent); background: var(--accent-bg); }
  .hdr-btn.logout:hover { border-color: var(--error); color: var(--error); }

  /* Chat area */
  .chat { flex: 1; overflow-y: auto; padding: 24px; display: flex; flex-direction: column; gap: 16px; }
  .chat::-webkit-scrollbar { width: 6px; }
  .chat::-webkit-scrollbar-thumb { background: var(--scrollbar); border-radius: 3px; }
  .msg { max-width: 80%; padding: 12px 16px; border-radius: 12px; line-height: 1.6; font-size: 14px; word-wrap: break-word; position: relative; }
  .msg.user { align-self: flex-end; background: var(--user-bg); color: var(--text); border-bottom-right-radius: 4px; white-space: pre-wrap; }
  .msg.assistant { align-self: flex-start; background: var(--bg3); border: 1px solid var(--border); border-bottom-left-radius: 4px; }
  .msg.system { align-self: center; background: transparent; color: var(--text3); font-size: 12px; font-style: italic; }

  /* Markdown inside assistant messages */
  .msg.assistant p { margin: 0.5em 0; }
  .msg.assistant p:first-child { margin-top: 0; }
  .msg.assistant p:last-child { margin-bottom: 0; }
  .msg.assistant ul, .msg.assistant ol { margin: 0.5em 0; padding-left: 1.5em; }
  .msg.assistant li { margin: 0.2em 0; }
  .msg.assistant h1, .msg.assistant h2, .msg.assistant h3 { margin: 0.8em 0 0.4em; color: var(--accent); }
  .msg.assistant h1 { font-size: 1.3em; }
  .msg.assistant h2 { font-size: 1.15em; }
  .msg.assistant h3 { font-size: 1.05em; }
  .msg.assistant blockquote { border-left: 3px solid var(--accent); padding-left: 12px; margin: 0.5em 0; color: var(--text2); }
  .msg.assistant a { color: var(--accent); text-decoration: underline; }
  .msg.assistant table { border-collapse: collapse; margin: 0.5em 0; font-size: 13px; }
  .msg.assistant th, .msg.assistant td { border: 1px solid var(--border2); padding: 6px 10px; }
  .msg.assistant th { background: var(--bg2); }
  .msg.assistant hr { border: none; border-top: 1px solid var(--border); margin: 1em 0; }

  /* Code blocks */
  .msg.assistant code { background: var(--code-bg); padding: 2px 6px; border-radius: 4px; font-size: 13px; font-family: 'SF Mono', 'Fira Code', monospace; }
  .msg.assistant pre { background: var(--code-bg); padding: 12px; border-radius: 8px; overflow-x: auto; margin: 8px 0; position: relative; }
  .msg.assistant pre code { background: none; padding: 0; }
  .copy-btn { position: absolute; top: 6px; right: 6px; background: var(--bg2); border: 1px solid var(--border2); border-radius: 4px; padding: 3px 8px; color: var(--text2); font-size: 11px; cursor: pointer; opacity: 0; transition: opacity 0.15s; }
  pre:hover .copy-btn { opacity: 1; }
  .copy-btn:hover { color: var(--accent); border-color: var(--accent); }
  .copy-btn.copied { color: var(--success); border-color: var(--success); }

  /* Input area */
  .input-area { padding: 16px 24px; background: var(--bg2); border-top: 1px solid var(--border); display: flex; gap: 12px; align-items: flex-end; }
  .input-area textarea { flex: 1; background: var(--bg3); border: 1px solid var(--border2); border-radius: 8px; padding: 12px 16px; color: var(--text); font-size: 14px; font-family: inherit; resize: none; outline: none; min-height: 44px; max-height: 120px; }
  .input-area textarea:focus { border-color: var(--accent); }
  .input-area .btn-group { display: flex; gap: 6px; }
  .input-area button { background: var(--accent); color: #000; border: none; border-radius: 8px; padding: 10px 18px; font-size: 14px; font-weight: 600; cursor: pointer; transition: opacity 0.2s; }
  .input-area button:hover { opacity: 0.85; }
  .input-area button:disabled { opacity: 0.4; cursor: not-allowed; }
  .upload-btn { background: var(--bg3) !important; color: var(--text2) !important; border: 1px solid var(--border2) !important; padding: 10px 12px !important; font-size: 16px !important; font-weight: 400 !important; }
  .upload-btn:hover { border-color: var(--accent) !important; color: var(--accent) !important; }

  /* Typing indicator */
  .typing { display: flex; gap: 4px; padding: 4px 0; }
  .typing span { width: 6px; height: 6px; background: var(--text3); border-radius: 50%; animation: bounce 1.4s infinite; }
  .typing span:nth-child(2) { animation-delay: 0.2s; }
  .typing span:nth-child(3) { animation-delay: 0.4s; }
  @keyframes bounce { 0%, 80%, 100% { transform: translateY(0); } 40% { transform: translateY(-8px); } }

  /* Welcome screen */
  .welcome { text-align: center; padding: 60px 24px; }
  .welcome h2 { color: var(--accent); margin-bottom: 8px; }
  .welcome p { color: var(--text3); font-size: 14px; max-width: 400px; margin: 0 auto; }
  .welcome .shortcuts { margin-top: 24px; font-size: 12px; color: var(--text4); }
  .welcome .shortcuts kbd { background: var(--bg3); border: 1px solid var(--border2); border-radius: 4px; padding: 2px 6px; font-size: 11px; font-family: monospace; }

  /* Login overlay */
  .login-overlay { position: fixed; inset: 0; background: var(--bg); display: flex; align-items: center; justify-content: center; z-index: 100; }
  .login-box { background: var(--bg2); border: 1px solid var(--border); border-radius: 12px; padding: 40px; max-width: 380px; width: 90%; text-align: center; }
  .login-box .logo { font-size: 32px; font-weight: 700; color: var(--accent); margin-bottom: 8px; }
  .login-box .tagline { font-size: 14px; color: var(--text3); margin-bottom: 28px; }
  .login-box label { display: block; text-align: left; font-size: 13px; color: var(--text2); margin-bottom: 6px; }
  .login-box input { width: 100%; background: var(--bg3); border: 1px solid var(--border2); border-radius: 8px; padding: 12px 16px; color: var(--text); font-size: 14px; font-family: 'SF Mono', 'Fira Code', monospace; outline: none; margin-bottom: 16px; }
  .login-box input:focus { border-color: var(--accent); }
  .login-box button { width: 100%; background: var(--accent); color: #000; border: none; border-radius: 8px; padding: 12px; font-size: 14px; font-weight: 600; cursor: pointer; }
  .login-box button:hover { opacity: 0.85; }
  .login-box button:disabled { opacity: 0.4; cursor: not-allowed; }
  .login-error { color: var(--error); font-size: 13px; margin-bottom: 12px; min-height: 20px; }
  .login-hint { font-size: 12px; color: var(--text4); margin-top: 16px; line-height: 1.5; }
  .login-hint code { background: var(--bg3); padding: 2px 6px; border-radius: 4px; font-size: 11px; }

  /* Slide-out panels (logs, settings) */
  .panel { position: fixed; top: 0; right: -500px; width: 500px; height: 100vh; background: var(--bg); border-left: 1px solid var(--border); z-index: 50; display: flex; flex-direction: column; transition: right 0.25s ease; }
  .panel.open { right: 0; }
  .panel-header { padding: 16px 20px; background: var(--bg2); border-bottom: 1px solid var(--border); display: flex; align-items: center; justify-content: space-between; }
  .panel-header h3 { font-size: 14px; font-weight: 600; color: var(--text); }
  .panel-header-btns { display: flex; gap: 8px; }
  .panel-header button { background: none; border: 1px solid var(--border2); border-radius: 6px; padding: 4px 10px; color: var(--text2); font-size: 11px; cursor: pointer; transition: all 0.15s; }
  .panel-header button:hover { border-color: var(--accent); color: var(--accent); }
  .panel-content { flex: 1; overflow-y: auto; padding: 16px 20px; }
  .panel-content::-webkit-scrollbar { width: 6px; }
  .panel-content::-webkit-scrollbar-thumb { background: var(--scrollbar); border-radius: 3px; }
  .panel-footer { padding: 8px 16px; background: var(--bg2); border-top: 1px solid var(--border); font-size: 11px; color: var(--text4); display: flex; align-items: center; justify-content: space-between; }

  /* Logs panel specific */
  .logs-content { font-family: 'SF Mono', 'Fira Code', monospace; font-size: 12px; line-height: 1.7; }
  .log-line { padding: 2px 0; border-bottom: 1px solid rgba(128,128,128,0.08); }
  .log-ts { color: var(--text4); margin-right: 8px; }
  .log-level { font-weight: 600; margin-right: 8px; }
  .log-level.INFO { color: var(--success); }
  .log-level.WARN { color: var(--warn); }
  .log-level.ERROR { color: var(--error); }
  .log-level.DEBUG { color: var(--text3); }
  .log-msg { color: var(--text); }
  .panel-empty { color: var(--text4); text-align: center; padding: 40px 20px; font-size: 13px; }
  .auto-refresh { display: flex; align-items: center; gap: 6px; }
  .auto-refresh input { accent-color: var(--accent); }

  /* Settings panel specific */
  .setting-group { margin-bottom: 20px; }
  .setting-group h4 { font-size: 12px; font-weight: 600; color: var(--text2); text-transform: uppercase; letter-spacing: 0.5px; margin-bottom: 10px; }
  .setting-row { margin-bottom: 12px; }
  .setting-row label { display: block; font-size: 12px; color: var(--text2); margin-bottom: 4px; }
  .setting-row input, .setting-row select, .setting-row textarea { width: 100%; background: var(--bg3); border: 1px solid var(--border2); border-radius: 6px; padding: 8px 12px; color: var(--text); font-size: 13px; outline: none; font-family: inherit; }
  .setting-row input:focus, .setting-row select:focus, .setting-row textarea:focus { border-color: var(--accent); }
  .setting-row textarea { min-height: 80px; resize: vertical; }
  .setting-row select { cursor: pointer; }
  .setting-actions { display: flex; gap: 8px; margin-top: 16px; }
  .setting-actions button { padding: 8px 16px; border-radius: 6px; font-size: 13px; font-weight: 500; cursor: pointer; transition: all 0.15s; }
  .btn-primary { background: var(--accent); color: #000; border: none; }
  .btn-primary:hover { opacity: 0.85; }
  .btn-secondary { background: var(--bg3); color: var(--text2); border: 1px solid var(--border2); }
  .btn-secondary:hover { border-color: var(--accent); color: var(--accent); }
  .setting-note { font-size: 11px; color: var(--text4); margin-top: 4px; }
  .setting-saved { color: var(--success); font-size: 12px; display: none; }

  /* Version/upgrade section */
  .version-info { padding: 16px; background: var(--bg3); border-radius: 8px; margin-bottom: 16px; }
  .version-info .ver-row { display: flex; justify-content: space-between; align-items: center; margin-bottom: 6px; font-size: 13px; }
  .version-info .ver-row:last-child { margin-bottom: 0; }
  .version-info .ver-label { color: var(--text2); }
  .version-info .ver-value { color: var(--text); font-family: monospace; }
  .update-badge { background: var(--accent-bg); color: var(--accent); padding: 2px 8px; border-radius: 4px; font-size: 11px; font-weight: 600; }
  .upgrade-btn { width: 100%; padding: 10px; background: var(--accent); color: #000; border: none; border-radius: 8px; font-size: 13px; font-weight: 600; cursor: pointer; margin-top: 12px; }
  .upgrade-btn:hover { opacity: 0.85; }
  .upgrade-btn:disabled { opacity: 0.4; cursor: not-allowed; }
  .upgrade-status { font-size: 12px; color: var(--text2); margin-top: 8px; text-align: center; }

  /* Backdrop */
  .backdrop { position: fixed; inset: 0; background: rgba(0,0,0,0.5); z-index: 49; opacity: 0; pointer-events: none; transition: opacity 0.25s ease; }
  .backdrop.open { opacity: 1; pointer-events: auto; }

  .hidden { display: none !important; }

  /* Mobile */
  @media (max-width: 768px) {
    .sidebar { position: fixed; left: 0; top: 0; height: 100vh; z-index: 60; }
    .sidebar.collapsed { margin-left: 0; transform: translateX(-100%); }
    .panel { width: 100%; right: -100%; }
    .msg { max-width: 95%; }
    .input-area { padding: 12px 16px; }
  }
</style>
</head>
<body>

<!-- Login overlay -->
<div class="login-overlay" id="loginOverlay">
  <div class="login-box">
    <div class="logo">&#x1fa99; PennyClaw</div>
    <div class="tagline">$0/month AI agent on GCP free tier</div>
    <div id="loginLoading" style="color: var(--text3); font-size: 13px;">Checking authentication...</div>
    <div id="loginForm" class="hidden">
      <label for="tokenInput">Authentication Token</label>
      <input type="password" id="tokenInput" placeholder="Enter your PENNYCLAW_AUTH_TOKEN" autocomplete="off" />
      <div class="login-error" id="loginError"></div>
      <button id="loginBtn" onclick="doLogin()">Sign In</button>
      <div class="login-hint">This is the value of your <code>PENNYCLAW_AUTH_TOKEN</code> environment variable.</div>
    </div>
  </div>
</div>

<!-- Sidebar -->
<div class="sidebar" id="sidebar">
  <div class="sidebar-header">
    <span class="brand">&#x1fa99; PennyClaw</span>
    <button onclick="toggleSidebar()" title="Collapse sidebar">&#x2190;</button>
  </div>
  <button class="new-chat-btn" onclick="newChat()">+ New Chat</button>
  <div class="session-list" id="sessionList"></div>
  <div class="sidebar-footer">
    <div class="token-display" id="tokenDisplay"></div>
    <button onclick="openPanel('settings')">&#x2699; Settings</button>
    <button onclick="openPanel('logs')">&#x1f4cb; Logs</button>
    <label style="display:flex;align-items:center;gap:6px;padding:6px 8px;font-size:12px;color:var(--text2);cursor:pointer;"><input type="checkbox" id="notifToggle" style="accent-color:var(--accent);" /> Sound notifications</label>
    <button id="logoutBtnSidebar" class="hidden" onclick="doLogout()">&#x1f6aa; Sign Out</button>
  </div>
</div>

<!-- Main area -->
<div class="main">
  <div class="header">
    <button class="toggle-sidebar" onclick="toggleSidebar()" title="Toggle sidebar">&#x2630;</button>
    <div class="title" id="chatTitle">New Chat</div>
    <div class="status">Online</div>
    <button class="hdr-btn" id="themeBtn" onclick="toggleTheme()" title="Toggle theme">&#x263E;</button>
    <button class="hdr-btn" id="exportBtn" onclick="exportChat()" title="Export chat (Ctrl+E)">Export</button>
  </div>
  <div class="chat" id="chat">
    <div class="welcome">
      <h2>Welcome to PennyClaw</h2>
      <p>Your $0/month personal AI agent, running on GCP's free tier. Type a message to get started.</p>
      <div class="shortcuts">
        <kbd>Ctrl</kbd>+<kbd>K</kbd> New chat &nbsp;
        <kbd>Ctrl</kbd>+<kbd>L</kbd> Clear &nbsp;
        <kbd>Ctrl</kbd>+<kbd>E</kbd> Export &nbsp;
        <kbd>Esc</kbd> Close panels
      </div>
    </div>
  </div>
  <div class="input-area">
    <textarea id="input" placeholder="Type a message... (Shift+Enter for newline)" rows="1"></textarea>
    <div class="btn-group">
      <button class="upload-btn" onclick="document.getElementById('fileInput').click()" title="Upload file">&#x1f4ce;</button>
      <button id="send" onclick="sendMessage()">Send</button>
    </div>
    <input type="file" id="fileInput" class="hidden" onchange="uploadFile(this)" />
  </div>
</div>

<!-- Backdrop for panels -->
<div class="backdrop" id="backdrop" onclick="closeAllPanels()"></div>

<!-- Logs panel -->
<div class="panel" id="logsPanel">
  <div class="panel-header">
    <h3>Application Logs</h3>
    <div class="panel-header-btns">
      <button onclick="fetchLogs()">Refresh</button>
      <button onclick="closeAllPanels()">Close</button>
    </div>
  </div>
  <div class="panel-content logs-content" id="logsContent">
    <div class="panel-empty">Loading logs...</div>
  </div>
  <div class="panel-footer">
    <span id="logsCount">0 entries</span>
    <div class="auto-refresh">
      <input type="checkbox" id="autoRefresh" checked />
      <label for="autoRefresh" style="cursor:pointer;">Auto-refresh (5s)</label>
    </div>
  </div>
</div>

<!-- Settings panel -->
<div class="panel" id="settingsPanel">
  <div class="panel-header">
    <h3>Settings</h3>
    <div class="panel-header-btns">
      <button onclick="closeAllPanels()">Close</button>
    </div>
  </div>
  <div class="panel-content" id="settingsContent">
    <!-- Version & Upgrade -->
    <div class="setting-group">
      <h4>Version</h4>
      <div class="version-info" id="versionInfo">
        <div class="ver-row"><span class="ver-label">Current</span><span class="ver-value" id="verCurrent">...</span></div>
        <div class="ver-row"><span class="ver-label">Latest</span><span class="ver-value" id="verLatest">...</span></div>
        <div class="ver-row"><span class="ver-label">Platform</span><span class="ver-value" id="verPlatform">...</span></div>
      </div>
      <button class="upgrade-btn" id="upgradeBtn" onclick="doUpgrade()" disabled>Check for Updates</button>
      <div class="upgrade-status" id="upgradeStatus"></div>
    </div>

    <!-- LLM Settings -->
    <div class="setting-group">
      <h4>LLM Configuration</h4>
      <div class="setting-row">
        <label>Provider</label>
        <select id="setProvider">
          <option value="openai">OpenAI</option>
          <option value="openai-compatible">OpenAI-Compatible</option>
          <option value="anthropic">Anthropic</option>
          <option value="gemini">Gemini</option>
        </select>
      </div>
      <div class="setting-row">
        <label>Model</label>
        <input type="text" id="setModel" placeholder="e.g., gpt-4.1-mini" />
      </div>
      <div class="setting-row">
        <label>API Key</label>
        <input type="password" id="setApiKey" placeholder="Enter API key (leave blank to keep current)" />
        <div class="setting-note">Masked for security. Enter a new key to update.</div>
      </div>
      <div class="setting-row">
        <label>Base URL (optional)</label>
        <input type="text" id="setBaseUrl" placeholder="https://api.openai.com/v1" />
        <div class="setting-note">Only needed for OpenAI-compatible providers.</div>
      </div>
      <div class="setting-row">
        <label>Max Tokens</label>
        <input type="number" id="setMaxTokens" min="100" max="128000" />
      </div>
      <div class="setting-row">
        <label>Temperature</label>
        <input type="number" id="setTemperature" min="0" max="2" step="0.1" />
      </div>
    </div>

    <!-- System Prompt -->
    <div class="setting-group">
      <h4>System Prompt</h4>
      <div class="setting-row">
        <textarea id="setSystemPrompt" rows="5" placeholder="System prompt for the agent..."></textarea>
      </div>
    </div>

    <div class="setting-actions">
      <button class="btn-primary" onclick="saveSettings()">Save Settings</button>
      <button class="btn-secondary" onclick="loadSettings()">Reset</button>
      <span class="setting-saved" id="settingSaved">&#x2713; Saved!</span>
    </div>
    <div class="setting-note" style="margin-top: 12px;">Some changes (provider, API key) require a server restart to take effect.</div>
  </div>
</div>

<script>
// ===== State =====
let sessionId = 'web-' + Date.now();
let isWelcome = true;
let authToken = localStorage.getItem('pennyclaw_token') || '';

// Auto-login from ?token= query param (used by deploy script one-click URL)
(function checkUrlToken() {
  const params = new URLSearchParams(window.location.search);
  const urlToken = params.get('token');
  if (urlToken) {
    authToken = urlToken;
    localStorage.setItem('pennyclaw_token', urlToken);
    // Strip token from URL bar so it doesn't linger in browser history
    const clean = window.location.pathname;
    window.history.replaceState({}, '', clean);
  }
})();
let logsInterval = null;
let currentPanel = null;
let notifEnabled = localStorage.getItem('pennyclaw_notif') !== 'false';
let sidebarOpen = localStorage.getItem('pennyclaw_sidebar') !== 'false';

// ===== DOM refs =====
const $ = id => document.getElementById(id);
const chat = $('chat');
const input = $('input');
const sendBtn = $('send');
const loginOverlay = $('loginOverlay');
const loginForm = $('loginForm');
const loginLoading = $('loginLoading');
const loginError = $('loginError');
const tokenInput = $('tokenInput');
const sidebar = $('sidebar');
const sessionList = $('sessionList');
const backdrop = $('backdrop');
const tokenDisplay = $('tokenDisplay');

// ===== Configure marked.js =====
if (typeof marked !== 'undefined') {
  marked.setOptions({ breaks: true, gfm: true });
  // Use marked-highlight extension if available (v15+ compatible)
  if (typeof markedHighlight !== 'undefined' && typeof hljs !== 'undefined') {
    marked.use(markedHighlight.markedHighlight({
      langPrefix: 'hljs language-',
      highlight: function(code, lang) {
        if (lang && hljs.getLanguage(lang)) {
          try { return hljs.highlight(code, { language: lang }).value; } catch (e) {}
        }
        try { return hljs.highlightAuto(code).value; } catch (e) {}
        return code;
      }
    }));
  }
}

// ===== Theme =====
function initTheme() {
  const saved = localStorage.getItem('pennyclaw_theme') || 'dark';
  document.documentElement.setAttribute('data-theme', saved);
  updateThemeBtn(saved);
  updateHljsTheme(saved);
}

function toggleTheme() {
  const current = document.documentElement.getAttribute('data-theme');
  const next = current === 'dark' ? 'light' : 'dark';
  document.documentElement.setAttribute('data-theme', next);
  localStorage.setItem('pennyclaw_theme', next);
  updateThemeBtn(next);
  updateHljsTheme(next);
}

function updateThemeBtn(theme) {
  $('themeBtn').textContent = theme === 'dark' ? '\u263E' : '\u2600';
}

function updateHljsTheme(theme) {
  $('hljs-theme-dark').disabled = theme !== 'dark';
  $('hljs-theme-light').disabled = theme !== 'light';
}

initTheme();

// ===== Sidebar =====
function toggleSidebar() {
  sidebarOpen = !sidebarOpen;
  sidebar.classList.toggle('collapsed', !sidebarOpen);
  localStorage.setItem('pennyclaw_sidebar', sidebarOpen);
}

if (!sidebarOpen) sidebar.classList.add('collapsed');

// ===== Notification toggle =====
const notifToggle = $('notifToggle');
notifToggle.checked = notifEnabled;
notifToggle.addEventListener('change', () => {
  notifEnabled = notifToggle.checked;
  localStorage.setItem('pennyclaw_notif', notifEnabled);
});

// ===== Auth =====
(async function checkAuth() {
  try {
    const headers = {};
    if (authToken) headers['Authorization'] = 'Bearer ' + authToken;
    const res = await fetch('/api/auth/check', { headers });
    const data = await res.json();
    if (!data.auth_required) {
      loginOverlay.classList.add('hidden');
      input.focus();
      loadSessions();
      startTokenTracking();
      return;
    }
    if (data.valid && authToken) {
      loginOverlay.classList.add('hidden');
      $('logoutBtnSidebar').classList.remove('hidden');
      input.focus();
      loadSessions();
      startTokenTracking();
      return;
    }
    localStorage.removeItem('pennyclaw_token');
    authToken = '';
    loginLoading.classList.add('hidden');
    loginForm.classList.remove('hidden');
    tokenInput.focus();
  } catch (err) {
    loginLoading.textContent = 'Cannot reach PennyClaw. Is it running?';
  }
})();

tokenInput.addEventListener('keydown', e => {
  if (e.key === 'Enter') { e.preventDefault(); doLogin(); }
});

async function doLogin() {
  const token = tokenInput.value.trim();
  if (!token) { loginError.textContent = 'Please enter a token.'; return; }
  loginError.textContent = '';
  $('loginBtn').disabled = true;
  try {
    const res = await fetch('/api/auth/check', { headers: { 'Authorization': 'Bearer ' + token } });
    const data = await res.json();
    if (data.valid) {
      authToken = token;
      localStorage.setItem('pennyclaw_token', token);
      loginOverlay.classList.add('hidden');
      $('logoutBtnSidebar').classList.remove('hidden');
      input.focus();
      loadSessions();
      startTokenTracking();
    } else {
      loginError.textContent = 'Invalid token. Check your PENNYCLAW_AUTH_TOKEN value.';
    }
  } catch (err) {
    loginError.textContent = 'Connection error. Is PennyClaw running?';
  }
  $('loginBtn').disabled = false;
}

function doLogout() {
  authToken = '';
  localStorage.removeItem('pennyclaw_token');
  location.reload();
}

// ===== API helper =====
function apiHeaders(extra) {
  const h = {};
  if (authToken) h['Authorization'] = 'Bearer ' + authToken;
  return Object.assign(h, extra || {});
}

async function apiFetch(url, opts) {
  opts = opts || {};
  opts.headers = apiHeaders(opts.headers);
  const res = await fetch(url, opts);
  if (res.status === 401) {
    addMessage('system', 'Session expired. Please sign in again.');
    doLogout();
    throw new Error('Unauthorized');
  }
  return res;
}

// ===== Panels =====
function openPanel(name) {
  closeAllPanels();
  currentPanel = name;
  const panel = $(name + 'Panel');
  panel.classList.add('open');
  backdrop.classList.add('open');
  if (name === 'logs') { fetchLogs(); startAutoRefresh(); }
  if (name === 'settings') { loadSettings(); checkVersion(); }
}

function closeAllPanels() {
  document.querySelectorAll('.panel').forEach(p => p.classList.remove('open'));
  backdrop.classList.remove('open');
  stopAutoRefresh();
  currentPanel = null;
}

// ===== Sessions =====
async function loadSessions() {
  try {
    const res = await apiFetch('/api/sessions');
    const data = await res.json();
    const sessions = data.sessions || [];
    renderSessions(sessions);
  } catch (e) {
    sessionList.innerHTML = '<div style="padding:12px;color:var(--text4);font-size:12px;">Could not load sessions.</div>';
  }
}

function renderSessions(sessions) {
  if (!sessions.length) {
    sessionList.innerHTML = '<div style="padding:12px;color:var(--text4);font-size:12px;">No conversations yet.</div>';
    return;
  }
  sessionList.innerHTML = sessions.map(s => {
    const active = s.id === sessionId ? ' active' : '';
    const preview = escapeHtml(s.preview || '(empty)');
    const safeId = escapeHtml(s.id);
    return '<div class="session-item' + active + '" data-sid="' + safeId + '">' +
      '<span class="preview-text">' + preview + '</span>' +
      '<button class="del-btn" data-del="' + safeId + '" title="Delete">&times;</button>' +
      '</div>';
  }).join('');
  // Event delegation for session clicks
  sessionList.querySelectorAll('.session-item').forEach(el => {
    el.addEventListener('click', () => switchSession(el.dataset.sid));
  });
  sessionList.querySelectorAll('.del-btn').forEach(el => {
    el.addEventListener('click', (e) => { e.stopPropagation(); deleteSession(el.dataset.del); });
  });
}

function newChat() {
  sessionId = 'web-' + Date.now();
  isWelcome = true;
  chat.innerHTML = '<div class="welcome"><h2>Welcome to PennyClaw</h2><p>Your $0/month personal AI agent. Type a message to get started.</p><div class="shortcuts"><kbd>Ctrl</kbd>+<kbd>K</kbd> New chat &nbsp;<kbd>Ctrl</kbd>+<kbd>L</kbd> Clear &nbsp;<kbd>Ctrl</kbd>+<kbd>E</kbd> Export &nbsp;<kbd>Esc</kbd> Close panels</div></div>';
  $('chatTitle').textContent = 'New Chat';
  input.focus();
  loadSessions();
}

async function switchSession(id) {
  sessionId = id;
  isWelcome = false;
  chat.innerHTML = '';
  // Try to get preview text from the session list item
  const sessionEl = sessionList.querySelector('[data-sid="' + CSS.escape(id) + '"] .preview-text');
  const preview = sessionEl ? sessionEl.textContent : '';
  $('chatTitle').textContent = preview && preview !== '(empty)' ? preview : 'Chat';
  try {
    const res = await apiFetch('/api/sessions/' + encodeURIComponent(id));
    const data = await res.json();
    (data.messages || []).forEach(m => addMessage(m.role, m.content));
  } catch (e) {
    addMessage('system', 'Failed to load session history.');
  }
  loadSessions();
}

async function deleteSession(id) {
  if (!confirm('Delete this conversation?')) return;
  try {
    await apiFetch('/api/sessions/' + encodeURIComponent(id), { method: 'DELETE' });
    if (id === sessionId) newChat();
    else loadSessions();
  } catch (e) {}
}

// ===== Logs =====
function startAutoRefresh() {
  stopAutoRefresh();
  if ($('autoRefresh').checked) logsInterval = setInterval(fetchLogs, 5000);
}
function stopAutoRefresh() {
  if (logsInterval) { clearInterval(logsInterval); logsInterval = null; }
}
$('autoRefresh').addEventListener('change', () => {
  if (currentPanel === 'logs') {
    if ($('autoRefresh').checked) startAutoRefresh();
    else stopAutoRefresh();
  }
});

async function fetchLogs() {
  try {
    const res = await apiFetch('/api/logs');
    const data = await res.json();
    const logs = data.logs || [];
    const el = $('logsContent');
    if (!logs.length) {
      el.innerHTML = '<div class="panel-empty">No log entries yet.</div>';
      $('logsCount').textContent = '0 entries';
      return;
    }
    const wasAtBottom = el.scrollTop + el.clientHeight >= el.scrollHeight - 20;
    el.innerHTML = logs.map(e => {
      const ts = e.timestamp.replace('T',' ').replace('Z','').substring(11,19);
      return '<div class="log-line"><span class="log-ts">' + ts + '</span><span class="log-level ' + e.level + '">' + e.level.padEnd(5) + '</span><span class="log-msg">' + escapeHtml(e.message) + '</span></div>';
    }).join('');
    $('logsCount').textContent = logs.length + ' entries';
    if (wasAtBottom) el.scrollTop = el.scrollHeight;
  } catch (e) {
    $('logsContent').innerHTML = '<div class="panel-empty">Failed to fetch logs.</div>';
  }
}

// ===== Settings =====
async function loadSettings() {
  try {
    const res = await apiFetch('/api/settings');
    const s = await res.json();
    $('setProvider').value = s.provider || 'openai';
    $('setModel').value = s.model || '';
    $('setApiKey').value = '';
    $('setApiKey').placeholder = s.api_key || 'Enter API key';
    $('setBaseUrl').value = s.base_url || '';
    $('setMaxTokens').value = s.max_tokens || 4096;
    $('setTemperature').value = s.temperature != null ? s.temperature : 0.7;
    $('setSystemPrompt').value = s.system_prompt || '';
  } catch (e) {}
}

async function saveSettings() {
  const body = {
    provider: $('setProvider').value,
    model: $('setModel').value,
    base_url: $('setBaseUrl').value,
    max_tokens: parseInt($('setMaxTokens').value) || 4096,
    temperature: parseFloat($('setTemperature').value) || 0.7,
    system_prompt: $('setSystemPrompt').value
  };
  const key = $('setApiKey').value.trim();
  if (key) body.api_key = key;
  try {
    const res = await apiFetch('/api/settings', {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(body)
    });
    const data = await res.json();
    const saved = $('settingSaved');
    saved.style.display = 'inline';
    setTimeout(() => saved.style.display = 'none', 3000);
  } catch (e) {
    alert('Failed to save settings.');
  }
}

// ===== Version & Upgrade =====
async function checkVersion() {
  $('upgradeBtn').disabled = true;
  $('upgradeBtn').textContent = 'Checking...';
  $('upgradeStatus').textContent = '';
  try {
    const res = await apiFetch('/api/version');
    const v = await res.json();
    $('verCurrent').textContent = v.current || 'unknown';
    $('verLatest').textContent = v.latest || 'unknown';
    $('verPlatform').textContent = (v.os || '?') + '/' + (v.arch || '?');
    if (v.update_available) {
      $('upgradeBtn').textContent = 'Upgrade to v' + v.latest;
      $('upgradeBtn').disabled = false;
    } else if (v.error) {
      $('upgradeBtn').textContent = 'Check Failed';
      $('upgradeStatus').textContent = v.error;
    } else {
      $('upgradeBtn').textContent = 'Up to Date';
      $('upgradeBtn').disabled = true;
    }
  } catch (e) {
    $('upgradeBtn').textContent = 'Check Failed';
    $('upgradeStatus').textContent = 'Could not reach server.';
  }
}

async function doUpgrade() {
  if (!confirm('Upgrade PennyClaw? The server will restart after upgrading.')) return;
  $('upgradeBtn').disabled = true;
  $('upgradeBtn').textContent = 'Upgrading...';
  $('upgradeStatus').textContent = 'Downloading update...';
  try {
    const res = await apiFetch('/api/upgrade', { method: 'POST' });
    const data = await res.json();
    $('upgradeStatus').textContent = data.message || 'Upgrade complete!';
    $('upgradeBtn').textContent = 'Upgraded!';
    if (data.status === 'upgraded') {
      $('upgradeStatus').textContent += ' Reloading in 5s...';
      setTimeout(() => location.reload(), 5000);
    }
  } catch (e) {
    $('upgradeStatus').textContent = 'Upgrade failed. Check logs for details.';
    $('upgradeBtn').textContent = 'Retry Upgrade';
    $('upgradeBtn').disabled = false;
  }
}

// ===== Token Usage =====
async function updateTokenDisplay() {
  try {
    const res = await apiFetch('/api/tokens');
    const t = await res.json();
    tokenDisplay.textContent = 'Tokens: ' + (t.total_tokens || 0).toLocaleString() + ' (' + (t.request_count || 0) + ' reqs)';
  } catch (e) {
    tokenDisplay.textContent = '';
  }
}
let tokenInterval = null;
function startTokenTracking() {
  updateTokenDisplay();
  if (!tokenInterval) tokenInterval = setInterval(updateTokenDisplay, 15000);
}

// ===== Export =====
async function exportChat() {
  if (isWelcome) return;
  try {
    const res = await apiFetch('/api/export?session_id=' + encodeURIComponent(sessionId) + '&format=markdown');
    const text = await res.text();
    const blob = new Blob([text], { type: 'text/markdown' });
    const url = URL.createObjectURL(blob);
    const a = document.createElement('a');
    a.href = url;
    a.download = 'pennyclaw-chat-' + sessionId + '.md';
    a.click();
    URL.revokeObjectURL(url);
  } catch (e) {
    // Fallback: export from DOM
    const msgs = chat.querySelectorAll('.msg');
    let md = '# PennyClaw Chat Export\n\n';
    msgs.forEach(m => {
      const role = m.classList.contains('user') ? 'User' : m.classList.contains('assistant') ? 'Assistant' : 'System';
      md += '### ' + role + '\n\n' + m.textContent + '\n\n';
    });
    const blob = new Blob([md], { type: 'text/markdown' });
    const url = URL.createObjectURL(blob);
    const a = document.createElement('a');
    a.href = url;
    a.download = 'pennyclaw-chat-' + sessionId + '.md';
    a.click();
    URL.revokeObjectURL(url);
  }
}

// ===== File Upload =====
async function uploadFile(input) {
  const file = input.files[0];
  if (!file) return;
  const formData = new FormData();
  formData.append('file', file);
  addMessage('system', 'Uploading ' + file.name + '...');
  try {
    const res = await fetch('/api/upload', {
      method: 'POST',
      headers: apiHeaders(),
      body: formData
    });
    const data = await res.json();
    if (data.status === 'ok') {
      addMessage('system', 'File uploaded: ' + data.filename + ' (' + formatBytes(data.size) + ')');
      // Send a message to the agent about the uploaded file
      const msg = 'I uploaded a file: ' + data.filename + ' (saved at ' + data.path + '). Please read and process it.';
      document.getElementById('input').value = msg;
    } else {
      addMessage('system', 'Upload failed.');
    }
  } catch (e) {
    addMessage('system', 'Upload failed: ' + e.message);
  }
  input.value = '';
}

function formatBytes(b) {
  if (b < 1024) return b + ' B';
  if (b < 1048576) return (b/1024).toFixed(1) + ' KB';
  return (b/1048576).toFixed(1) + ' MB';
}

// ===== Chat =====
function renderMarkdown(text) {
  if (typeof marked === 'undefined') return escapeHtml(text);
  try {
    let html = marked.parse(text);
    if (typeof DOMPurify !== 'undefined') {
      html = DOMPurify.sanitize(html, { ADD_ATTR: ['target'] });
    }
    return html;
  } catch (e) {
    return escapeHtml(text);
  }
}

function addMessage(role, content) {
  if (isWelcome) { chat.innerHTML = ''; isWelcome = false; }
  const div = document.createElement('div');
  div.className = 'msg ' + role;
  if (role === 'assistant') {
    div.innerHTML = renderMarkdown(content);
    // Add copy buttons to code blocks
    div.querySelectorAll('pre').forEach(pre => {
      const btn = document.createElement('button');
      btn.className = 'copy-btn';
      btn.textContent = 'Copy';
      btn.onclick = function() {
        const code = pre.querySelector('code');
        navigator.clipboard.writeText(code ? code.textContent : pre.textContent).then(() => {
          btn.textContent = 'Copied!';
          btn.classList.add('copied');
          setTimeout(() => { btn.textContent = 'Copy'; btn.classList.remove('copied'); }, 2000);
        });
      };
      pre.style.position = 'relative';
      pre.appendChild(btn);
    });
    // Open links in new tab
    div.querySelectorAll('a').forEach(a => { a.target = '_blank'; a.rel = 'noopener'; });
  } else {
    div.textContent = content;
  }
  chat.appendChild(div);
  chat.scrollTop = chat.scrollHeight;
  return div;
}

function showTyping() {
  const div = document.createElement('div');
  div.className = 'msg assistant';
  div.id = 'typing';
  div.innerHTML = '<div class="typing"><span></span><span></span><span></span></div>';
  chat.appendChild(div);
  chat.scrollTop = chat.scrollHeight;
}

function hideTyping() {
  const el = $('typing');
  if (el) el.remove();
}

// Notification sound using Web Audio API
function playNotifSound() {
  if (!notifEnabled) return;
  try {
    const ctx = new (window.AudioContext || window.webkitAudioContext)();
    const osc = ctx.createOscillator();
    const gain = ctx.createGain();
    osc.connect(gain);
    gain.connect(ctx.destination);
    osc.frequency.value = 660;
    osc.type = 'sine';
    gain.gain.setValueAtTime(0.1, ctx.currentTime);
    gain.gain.exponentialRampToValueAtTime(0.001, ctx.currentTime + 0.3);
    osc.start(ctx.currentTime);
    osc.stop(ctx.currentTime + 0.3);
  } catch (e) {}
}

input.addEventListener('keydown', e => {
  if (e.key === 'Enter' && !e.shiftKey) { e.preventDefault(); sendMessage(); }
});
input.addEventListener('input', () => {
  input.style.height = 'auto';
  input.style.height = Math.min(input.scrollHeight, 120) + 'px';
});

async function sendMessage() {
  const msg = input.value.trim();
  if (!msg) return;
  input.value = '';
  input.style.height = 'auto';
  sendBtn.disabled = true;
  addMessage('user', msg);
  showTyping();
  try {
    const res = await apiFetch('/api/chat', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ message: msg, session_id: sessionId })
    });
    const data = await res.json();
    hideTyping();
    addMessage('assistant', data.response);
    if (data.session_id) sessionId = data.session_id;
    playNotifSound();
    updateTokenDisplay();
    loadSessions();
  } catch (err) {
    hideTyping();
    if (err.message !== 'Unauthorized') {
      addMessage('system', 'Connection error. Is PennyClaw running?');
    }
  }
  sendBtn.disabled = false;
  input.focus();
}

// ===== Keyboard shortcuts =====
document.addEventListener('keydown', e => {
  // Ctrl+K: New chat
  if ((e.ctrlKey || e.metaKey) && e.key === 'k') { e.preventDefault(); newChat(); }
  // Ctrl+L: Clear chat
  if ((e.ctrlKey || e.metaKey) && e.key === 'l') { e.preventDefault(); newChat(); }
  // Ctrl+E: Export
  if ((e.ctrlKey || e.metaKey) && e.key === 'e') { e.preventDefault(); exportChat(); }
  // Escape: Close panels
  if (e.key === 'Escape') { closeAllPanels(); input.focus(); }
});

// ===== Utility =====
function escapeHtml(text) {
  const div = document.createElement('div');
  div.textContent = text;
  return div.innerHTML.replace(/'/g, '&#39;').replace(/"/g, '&quot;');
}

input.focus();
</script>
</body>
</html>
`
