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
  .header .status::before { content: ''; width: 7px; height: 7px; background: currentColor; border-radius: 50%; }
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

  /* Toast notifications */
  .toast-container { position: fixed; top: 16px; right: 16px; z-index: 200; display: flex; flex-direction: column; gap: 8px; }
  .toast { padding: 10px 16px; border-radius: 8px; font-size: 13px; color: var(--text); background: var(--bg2); border: 1px solid var(--border2); box-shadow: 0 4px 12px rgba(0,0,0,0.3); animation: toastIn 0.25s ease; max-width: 360px; display: flex; align-items: center; gap: 8px; }
  .toast.success { border-color: var(--success); }
  .toast.error { border-color: var(--error); }
  .toast.info { border-color: var(--accent); }
  .toast-icon { font-size: 16px; flex-shrink: 0; }
  @keyframes toastIn { from { opacity: 0; transform: translateX(20px); } to { opacity: 1; transform: translateX(0); } }

  /* Health panel */
  .health-grid { display: grid; grid-template-columns: 1fr 1fr; gap: 12px; margin-bottom: 16px; }
  .health-card { background: var(--bg3); border: 1px solid var(--border); border-radius: 8px; padding: 14px; }
  .health-card .hc-label { font-size: 11px; color: var(--text3); text-transform: uppercase; letter-spacing: 0.5px; margin-bottom: 6px; }
  .health-card .hc-value { font-size: 22px; font-weight: 700; color: var(--text); font-family: 'SF Mono', monospace; }
  .health-card .hc-sub { font-size: 11px; color: var(--text4); margin-top: 4px; }
  .gauge-bar { height: 6px; background: var(--bg); border-radius: 3px; margin-top: 8px; overflow: hidden; }
  .gauge-fill { height: 100%; border-radius: 3px; transition: width 0.5s ease; }
  .gauge-fill.ok { background: var(--success); }
  .gauge-fill.warn { background: var(--warn); }
  .gauge-fill.crit { background: var(--error); }
  .health-status-banner { padding: 10px 14px; border-radius: 8px; font-size: 13px; font-weight: 500; margin-bottom: 16px; display: flex; align-items: center; gap: 8px; }
  .health-status-banner.healthy { background: rgba(76,175,80,0.1); color: var(--success); border: 1px solid rgba(76,175,80,0.2); }
  .health-status-banner.degraded { background: rgba(245,166,35,0.1); color: var(--warn); border: 1px solid rgba(245,166,35,0.2); }
  .health-status-banner.unhealthy { background: rgba(239,68,68,0.1); color: var(--error); border: 1px solid rgba(239,68,68,0.2); }
  .health-section { margin-bottom: 16px; }
  .health-section h4 { font-size: 12px; font-weight: 600; color: var(--text2); text-transform: uppercase; letter-spacing: 0.5px; margin-bottom: 10px; }
  .health-row { display: flex; justify-content: space-between; align-items: center; padding: 6px 0; font-size: 13px; border-bottom: 1px solid var(--border); }
  .health-row:last-child { border-bottom: none; }
  .health-row .hr-label { color: var(--text2); }
  .health-row .hr-value { color: var(--text); font-family: monospace; font-size: 12px; }

  /* Task panel */
  .task-add-bar { display: flex; gap: 8px; margin-bottom: 16px; }
  .task-add-bar input { flex: 1; background: var(--bg3); border: 1px solid var(--border2); border-radius: 6px; padding: 8px 12px; color: var(--text); font-size: 13px; outline: none; }
  .task-add-bar input:focus { border-color: var(--accent); }
  .task-add-bar button { background: var(--accent); color: #000; border: none; border-radius: 6px; padding: 8px 14px; font-size: 13px; font-weight: 600; cursor: pointer; white-space: nowrap; }
  .task-add-bar button:hover { opacity: 0.85; }
  .task-filters { display: flex; gap: 6px; margin-bottom: 12px; flex-wrap: wrap; }
  .task-filter { background: var(--bg3); border: 1px solid var(--border2); border-radius: 6px; padding: 4px 10px; font-size: 11px; color: var(--text2); cursor: pointer; transition: all 0.15s; }
  .task-filter:hover { border-color: var(--accent); color: var(--accent); }
  .task-filter.active { background: var(--accent-bg); border-color: var(--accent); color: var(--accent); }
  .task-card { background: var(--bg3); border: 1px solid var(--border); border-radius: 8px; padding: 12px; margin-bottom: 8px; transition: all 0.15s; }
  .task-card:hover { border-color: var(--border2); }
  .task-card.done { opacity: 0.5; }
  .task-card-header { display: flex; align-items: center; gap: 8px; }
  .task-check { width: 18px; height: 18px; border-radius: 4px; border: 2px solid var(--border2); cursor: pointer; display: flex; align-items: center; justify-content: center; flex-shrink: 0; transition: all 0.15s; font-size: 11px; }
  .task-check:hover { border-color: var(--accent); }
  .task-check.checked { background: var(--success); border-color: var(--success); color: #fff; }
  .task-title { flex: 1; font-size: 13px; color: var(--text); }
  .task-card.done .task-title { text-decoration: line-through; color: var(--text3); }
  .task-priority { font-size: 10px; font-weight: 600; padding: 2px 6px; border-radius: 4px; text-transform: uppercase; }
  .task-priority.high { background: rgba(239,68,68,0.15); color: var(--error); }
  .task-priority.medium { background: rgba(245,166,35,0.15); color: var(--warn); }
  .task-priority.low { background: rgba(76,175,80,0.15); color: var(--success); }
  .task-meta { display: flex; gap: 8px; margin-top: 6px; padding-left: 26px; flex-wrap: wrap; }
  .task-tag { font-size: 10px; background: var(--accent-bg); color: var(--accent); padding: 1px 6px; border-radius: 3px; }
  .task-due { font-size: 11px; color: var(--text3); }
  .task-actions { display: flex; gap: 4px; flex-shrink: 0; }
  .task-actions button { background: none; border: none; color: var(--text4); cursor: pointer; font-size: 14px; padding: 2px; transition: color 0.15s; }
  .task-actions button:hover { color: var(--error); }
  .task-empty { text-align: center; padding: 40px 20px; color: var(--text4); font-size: 13px; }

  /* Notes panel */
  .notes-layout { display: flex; flex-direction: column; height: 100%; }
  .notes-toolbar { display: flex; gap: 8px; margin-bottom: 12px; }
  .notes-toolbar input { flex: 1; background: var(--bg3); border: 1px solid var(--border2); border-radius: 6px; padding: 8px 12px; color: var(--text); font-size: 13px; outline: none; }
  .notes-toolbar input:focus { border-color: var(--accent); }
  .notes-toolbar button { background: var(--accent); color: #000; border: none; border-radius: 6px; padding: 8px 14px; font-size: 13px; font-weight: 600; cursor: pointer; white-space: nowrap; }
  .notes-toolbar button:hover { opacity: 0.85; }
  .notes-list { margin-bottom: 12px; }
  .note-item { display: flex; align-items: center; justify-content: space-between; padding: 8px 12px; border-radius: 6px; cursor: pointer; font-size: 13px; color: var(--text2); transition: all 0.15s; margin-bottom: 2px; }
  .note-item:hover { background: var(--bg3); color: var(--text); }
  .note-item.active { background: var(--accent-bg); color: var(--accent); }
  .note-item-info { display: flex; flex-direction: column; flex: 1; min-width: 0; }
  .note-item-name { font-weight: 500; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .note-item-meta { font-size: 11px; color: var(--text4); }
  .note-item-actions { display: flex; gap: 4px; }
  .note-item-actions button { background: none; border: none; color: var(--text4); cursor: pointer; font-size: 14px; padding: 2px; }
  .note-item-actions button:hover { color: var(--error); }
  .note-editor-area { flex: 1; display: flex; flex-direction: column; }
  .note-editor-header { display: flex; align-items: center; gap: 8px; margin-bottom: 8px; }
  .note-editor-header input { flex: 1; background: var(--bg3); border: 1px solid var(--border2); border-radius: 6px; padding: 6px 10px; color: var(--text); font-size: 13px; font-weight: 500; outline: none; }
  .note-editor-header input:focus { border-color: var(--accent); }
  .note-editor-tabs { display: flex; gap: 4px; margin-bottom: 8px; }
  .note-editor-tab { padding: 4px 10px; border-radius: 4px; font-size: 11px; color: var(--text3); cursor: pointer; transition: all 0.15s; }
  .note-editor-tab:hover { color: var(--text); }
  .note-editor-tab.active { background: var(--accent-bg); color: var(--accent); }
  .note-textarea { flex: 1; background: var(--bg3); border: 1px solid var(--border2); border-radius: 6px; padding: 12px; color: var(--text); font-size: 13px; font-family: 'SF Mono', 'Fira Code', monospace; outline: none; resize: none; min-height: 200px; }
  .note-textarea:focus { border-color: var(--accent); }
  .note-preview { flex: 1; background: var(--bg3); border: 1px solid var(--border); border-radius: 6px; padding: 12px; overflow-y: auto; min-height: 200px; font-size: 13px; line-height: 1.6; }
  .note-preview h1, .note-preview h2, .note-preview h3 { color: var(--accent); margin: 0.5em 0 0.3em; }
  .note-preview p { margin: 0.4em 0; }
  .note-preview code { background: var(--code-bg); padding: 2px 6px; border-radius: 4px; font-size: 12px; }
  .note-preview pre { background: var(--code-bg); padding: 10px; border-radius: 6px; overflow-x: auto; margin: 8px 0; }
  .note-preview pre code { background: none; padding: 0; }
  .note-empty { text-align: center; padding: 40px 20px; color: var(--text4); font-size: 13px; }

  /* Workspace panel */
  .ws-file-list { margin-bottom: 12px; }
  .ws-file { display: flex; align-items: center; justify-content: space-between; padding: 8px 12px; border-radius: 6px; cursor: pointer; font-size: 13px; color: var(--text2); transition: all 0.15s; margin-bottom: 2px; }
  .ws-file:hover { background: var(--bg3); color: var(--text); }
  .ws-file.active { background: var(--accent-bg); color: var(--accent); }
  .ws-file-name { font-family: 'SF Mono', monospace; font-size: 12px; flex: 1; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .ws-file-actions { display: flex; gap: 4px; }
  .ws-file-actions button { background: none; border: none; color: var(--text4); cursor: pointer; font-size: 14px; padding: 2px; }
  .ws-file-actions button:hover { color: var(--error); }
  .ws-editor { flex: 1; display: flex; flex-direction: column; }
  .ws-editor-header { display: flex; align-items: center; gap: 8px; margin-bottom: 8px; font-size: 13px; color: var(--text2); }
  .ws-editor-header strong { color: var(--text); font-family: monospace; }
  .ws-editor textarea { flex: 1; background: var(--bg3); border: 1px solid var(--border2); border-radius: 6px; padding: 12px; color: var(--text); font-size: 12px; font-family: 'SF Mono', 'Fira Code', monospace; outline: none; resize: none; min-height: 300px; line-height: 1.6; }
  .ws-editor textarea:focus { border-color: var(--accent); }
  .ws-bootstrap-banner { padding: 10px 14px; border-radius: 8px; background: rgba(245,166,35,0.1); border: 1px solid rgba(245,166,35,0.2); color: var(--warn); font-size: 12px; margin-bottom: 12px; display: flex; align-items: center; gap: 8px; }
  .ws-bootstrap-banner button { background: var(--accent); color: #000; border: none; border-radius: 4px; padding: 4px 10px; font-size: 11px; font-weight: 600; cursor: pointer; }

  /* Cron panel */
  .cron-job-card { background: var(--bg3); border: 1px solid var(--border); border-radius: 8px; padding: 12px; margin-bottom: 8px; }
  .cron-job-header { display: flex; align-items: center; gap: 8px; margin-bottom: 6px; }
  .cron-job-name { font-size: 13px; font-weight: 600; color: var(--text); flex: 1; }
  .cron-job-badge { font-size: 10px; font-weight: 600; padding: 2px 6px; border-radius: 4px; text-transform: uppercase; }
  .cron-job-badge.enabled { background: rgba(76,175,80,0.15); color: var(--success); }
  .cron-job-badge.disabled { background: rgba(128,128,128,0.15); color: var(--text3); }
  .cron-job-schedule { font-size: 12px; color: var(--text2); font-family: monospace; margin-bottom: 4px; }
  .cron-job-message { font-size: 12px; color: var(--text3); margin-bottom: 6px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .cron-job-meta { display: flex; gap: 12px; font-size: 11px; color: var(--text4); }
  .cron-job-actions { display: flex; gap: 4px; }
  .cron-job-actions button { background: none; border: 1px solid var(--border2); border-radius: 4px; padding: 3px 8px; font-size: 11px; color: var(--text2); cursor: pointer; transition: all 0.15s; }
  .cron-job-actions button:hover { border-color: var(--accent); color: var(--accent); }
  .cron-job-actions button.danger:hover { border-color: var(--error); color: var(--error); }
  .cron-form { background: var(--bg3); border: 1px solid var(--border); border-radius: 8px; padding: 16px; margin-bottom: 16px; }
  .cron-form h4 { font-size: 12px; font-weight: 600; color: var(--text2); margin-bottom: 12px; }
  .cron-form-row { margin-bottom: 10px; }
  .cron-form-row label { display: block; font-size: 11px; color: var(--text3); margin-bottom: 4px; }
  .cron-form-row input, .cron-form-row select, .cron-form-row textarea { width: 100%; background: var(--bg); border: 1px solid var(--border2); border-radius: 6px; padding: 6px 10px; color: var(--text); font-size: 12px; outline: none; font-family: inherit; }
  .cron-form-row input:focus, .cron-form-row select:focus, .cron-form-row textarea:focus { border-color: var(--accent); }
  .cron-form-row textarea { min-height: 60px; resize: vertical; }
  .cron-form-actions { display: flex; gap: 8px; }
  .cron-runs { margin-top: 8px; }
  .cron-run { display: flex; align-items: center; gap: 8px; padding: 4px 0; font-size: 11px; border-bottom: 1px solid var(--border); }
  .cron-run:last-child { border-bottom: none; }
  .cron-run-status { font-weight: 600; }
  .cron-run-status.success { color: var(--success); }
  .cron-run-status.error { color: var(--error); }
  .cron-run-status.running { color: var(--accent); }
  .cron-run-time { color: var(--text4); }
  .cron-run-result { color: var(--text3); flex: 1; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }

  /* Skills panel */
  .skill-card { background: var(--bg3); border: 1px solid var(--border); border-radius: 8px; padding: 12px; margin-bottom: 8px; }
  .skill-card-header { display: flex; align-items: center; gap: 8px; margin-bottom: 4px; }
  .skill-name { font-size: 13px; font-weight: 600; color: var(--text); flex: 1; }
  .skill-version { font-size: 10px; color: var(--text4); font-family: monospace; }
  .skill-badge { font-size: 10px; font-weight: 600; padding: 2px 6px; border-radius: 4px; }
  .skill-badge.bundled { background: rgba(245,166,35,0.15); color: var(--accent); }
  .skill-badge.installed { background: rgba(76,175,80,0.15); color: var(--success); }
  .skill-desc { font-size: 12px; color: var(--text3); margin-bottom: 6px; }
  .skill-author { font-size: 11px; color: var(--text4); }
  .skill-actions { display: flex; gap: 4px; align-items: center; }
  .skill-toggle { position: relative; width: 36px; height: 20px; cursor: pointer; }
  .skill-toggle input { opacity: 0; width: 0; height: 0; }
  .skill-toggle .slider { position: absolute; inset: 0; background: var(--border2); border-radius: 10px; transition: 0.2s; }
  .skill-toggle .slider::before { content: ''; position: absolute; width: 16px; height: 16px; border-radius: 50%; background: var(--text); bottom: 2px; left: 2px; transition: 0.2s; }
  .skill-toggle input:checked + .slider { background: var(--success); }
  .skill-toggle input:checked + .slider::before { transform: translateX(16px); }
  .skill-search-results { margin-top: 12px; }
  .skill-install-form { display: flex; gap: 8px; margin-bottom: 12px; }
  .skill-install-form input { flex: 1; background: var(--bg3); border: 1px solid var(--border2); border-radius: 6px; padding: 8px 12px; color: var(--text); font-size: 13px; outline: none; }
  .skill-install-form input:focus { border-color: var(--accent); }
  .skill-install-form button { background: var(--accent); color: #000; border: none; border-radius: 6px; padding: 8px 14px; font-size: 13px; font-weight: 600; cursor: pointer; white-space: nowrap; }
  .skill-tabs { display: flex; gap: 4px; margin-bottom: 12px; }
  .skill-tab { padding: 6px 12px; border-radius: 6px; font-size: 12px; color: var(--text2); cursor: pointer; transition: all 0.15s; }
  .skill-tab:hover { color: var(--text); }
  .skill-tab.active { background: var(--accent-bg); color: var(--accent); }

  /* Channel config cards */
  .channel-card { background: var(--bg3); border: 1px solid var(--border); border-radius: 8px; padding: 14px; margin-bottom: 10px; }
  .channel-card-header { display: flex; align-items: center; gap: 8px; margin-bottom: 8px; }
  .channel-card-icon { font-size: 18px; }
  .channel-card-name { font-size: 13px; font-weight: 600; color: var(--text); flex: 1; }
  .channel-card-status { font-size: 10px; font-weight: 600; padding: 2px 8px; border-radius: 4px; }
  .channel-card-status.connected { background: rgba(76,175,80,0.15); color: var(--success); }
  .channel-card-status.disconnected { background: rgba(128,128,128,0.15); color: var(--text3); }
  .channel-card-body { font-size: 12px; color: var(--text3); }
  .channel-card-body .setup-steps { margin: 8px 0; padding: 0; list-style: none; counter-reset: step; }
  .channel-card-body .setup-steps li { counter-increment: step; padding: 4px 0 4px 24px; position: relative; }
  .channel-card-body .setup-steps li::before { content: counter(step); position: absolute; left: 0; top: 4px; width: 18px; height: 18px; border-radius: 50%; background: var(--accent-bg); color: var(--accent); font-size: 10px; font-weight: 700; display: flex; align-items: center; justify-content: center; }
  .channel-card-body input { width: 100%; background: var(--bg); border: 1px solid var(--border2); border-radius: 6px; padding: 6px 10px; color: var(--text); font-size: 12px; outline: none; margin-top: 4px; }
  .channel-card-body input:focus { border-color: var(--accent); }
  .channel-card-actions { display: flex; gap: 6px; margin-top: 8px; }
  .channel-card-actions button { font-size: 11px; padding: 4px 10px; border-radius: 4px; cursor: pointer; border: 1px solid var(--border2); background: none; color: var(--text2); transition: all 0.15s; }
  .channel-card-actions button:hover { border-color: var(--accent); color: var(--accent); }
  .channel-card-actions button.primary { background: var(--accent); color: #000; border-color: var(--accent); }
  .channel-card-actions button.primary:hover { opacity: 0.9; }
  .channel-card-actions a { font-size: 11px; padding: 4px 10px; border-radius: 4px; cursor: pointer; border: 1px solid var(--border2); background: none; color: var(--text2); transition: all 0.15s; text-decoration: none; display: inline-flex; align-items: center; gap: 4px; }
  .channel-card-actions a:hover { border-color: var(--accent); color: var(--accent); }
  .webhook-url-box { display: flex; gap: 4px; margin-top: 6px; }
  .webhook-url-box input { flex: 1; font-family: monospace; font-size: 11px; }
  .webhook-url-box button { background: var(--bg3); border: 1px solid var(--border2); border-radius: 4px; padding: 4px 8px; font-size: 11px; color: var(--text2); cursor: pointer; white-space: nowrap; }
  .webhook-url-box button:hover { color: var(--accent); border-color: var(--accent); }

  /* Mobile responsive for new panels */
  @media (max-width: 768px) {
    .cron-job-meta { flex-wrap: wrap; }
    .cron-form-actions { flex-direction: column; }
    .skill-install-form { flex-direction: column; }
    .skill-install-form button { width: 100%; }
    .ws-editor textarea { min-height: 200px; }
    .channel-card-actions { flex-wrap: wrap; }
  }

  /* Sidebar tools section */
  .sidebar-tools { padding: 8px 16px; border-top: 1px solid var(--border); }
  .sidebar-tools-label { font-size: 10px; font-weight: 600; color: var(--text4); text-transform: uppercase; letter-spacing: 0.5px; padding: 4px 8px; }
  .sidebar-tools button { width: 100%; background: none; border: none; color: var(--text2); cursor: pointer; font-size: 12px; padding: 6px 8px; border-radius: 6px; text-align: left; transition: all 0.15s; display: flex; align-items: center; gap: 8px; }
  .sidebar-tools button:hover { background: var(--bg3); color: var(--text); }

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
  <div class="sidebar-tools">
    <div class="sidebar-tools-label">Tools</div>
    <button onclick="openPanel('health')">&#x1f4ca; Health</button>
    <button onclick="openPanel('tasks')">&#x2611; Tasks <span id="taskBadge" style="font-size:10px;background:var(--accent-bg);color:var(--accent);padding:1px 5px;border-radius:3px;margin-left:auto;"></span></button>
    <button onclick="openPanel('notes')">&#x1f4dd; Notes</button>
    <button onclick="openPanel('workspace')">&#x1f4c1; Workspace</button>
    <button onclick="openPanel('scheduler')">&#x23f0; Scheduler</button>
    <button onclick="openPanel('skills')">&#x1f9e9; Skills</button>
  </div>
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
    <div class="status" id="headerStatus" onclick="openPanel('health')" style="cursor:pointer;" title="Click for health details">Online</div>
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
        <kbd>Esc</kbd> Close panels<br>
        <kbd>Ctrl</kbd>+<kbd>Shift</kbd>+<kbd>H</kbd> Health &nbsp;
        <kbd>Ctrl</kbd>+<kbd>Shift</kbd>+<kbd>T</kbd> Tasks &nbsp;
        <kbd>Ctrl</kbd>+<kbd>Shift</kbd>+<kbd>N</kbd> Notes &nbsp;
        <kbd>Ctrl</kbd>+<kbd>Shift</kbd>+<kbd>W</kbd> Workspace &nbsp;
        <kbd>Ctrl</kbd>+<kbd>Shift</kbd>+<kbd>J</kbd> Scheduler<br>
        <kbd>Ctrl</kbd>+<kbd>S</kbd> Save note/file
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

    <!-- Channels & Integrations -->
    <div class="setting-group">
      <h4>Channels & Integrations</h4>

      <!-- Telegram -->
      <div class="channel-card" id="channelTelegram">
        <div class="channel-card-header">
          <span class="channel-card-icon">&#x2708;</span>
          <span class="channel-card-name">Telegram</span>
          <span class="channel-card-status disconnected" id="telegramStatus">Not Connected</span>
        </div>
        <div class="channel-card-body">
          <ol class="setup-steps">
            <li>Open Telegram and message <strong>@BotFather</strong></li>
            <li>Send <code>/newbot</code> and follow the prompts</li>
            <li>Copy the bot token and paste it below</li>
          </ol>
          <input type="password" id="setTelegramToken" placeholder="Paste your Telegram bot token here" />
          <div class="channel-card-actions">
            <a href="https://t.me/BotFather" target="_blank" rel="noopener">&#x2197; Open BotFather</a>
            <button class="primary" onclick="saveTelegramToken()">Save & Enable</button>
          </div>
        </div>
      </div>

      <!-- Email / SMTP -->
      <div class="channel-card" id="channelEmail">
        <div class="channel-card-header">
          <span class="channel-card-icon">&#x2709;</span>
          <span class="channel-card-name">Email (SMTP)</span>
          <span class="channel-card-status disconnected" id="emailStatus">Not Configured</span>
        </div>
        <div class="channel-card-body">
          <ol class="setup-steps">
            <li>For Gmail: enable 2FA, then create an <strong>App Password</strong></li>
            <li>Enter your SMTP details below</li>
          </ol>
          <div style="display:grid;grid-template-columns:1fr 1fr;gap:6px;margin-top:6px;">
            <input type="text" id="setSmtpHost" placeholder="SMTP host (smtp.gmail.com)" />
            <input type="number" id="setSmtpPort" placeholder="Port (587)" />
            <input type="text" id="setSmtpUser" placeholder="Username (you@gmail.com)" />
            <input type="password" id="setSmtpPass" placeholder="App Password" />
          </div>
          <div class="channel-card-actions">
            <a href="https://myaccount.google.com/apppasswords" target="_blank" rel="noopener">&#x2197; Gmail App Passwords</a>
            <button onclick="testEmail()">Test Email</button>
            <button class="primary" onclick="saveEmailConfig()">Save & Enable</button>
          </div>
        </div>
      </div>

      <!-- Webhook -->
      <div class="channel-card" id="channelWebhook">
        <div class="channel-card-header">
          <span class="channel-card-icon">&#x1f517;</span>
          <span class="channel-card-name">Webhook</span>
          <span class="channel-card-status disconnected" id="webhookStatus">Not Configured</span>
        </div>
        <div class="channel-card-body">
          <p>Receive events from external services (GitHub, Stripe, etc.) via HTTP POST.</p>
          <div class="webhook-url-box">
            <input type="text" id="webhookUrl" readonly />
            <button onclick="copyWebhookUrl()">Copy</button>
          </div>
          <div style="margin-top:6px;">
            <input type="password" id="setWebhookSecret" placeholder="HMAC secret (optional, for signature verification)" />
          </div>
          <div class="channel-card-actions">
            <button class="primary" onclick="saveWebhookConfig()">Save & Enable</button>
            <button onclick="testWebhook()">Send Test</button>
          </div>
        </div>
      </div>

      <!-- Discord -->
      <div class="channel-card" id="channelDiscord">
        <div class="channel-card-header">
          <span class="channel-card-icon">&#x1f3ae;</span>
          <span class="channel-card-name">Discord</span>
          <span class="channel-card-status disconnected" id="discordStatus">Not Connected</span>
        </div>
        <div class="channel-card-body">
          <ol class="setup-steps">
            <li>Go to the <strong>Discord Developer Portal</strong></li>
            <li>Create a new application and add a Bot</li>
            <li>Copy the bot token and paste it below</li>
          </ol>
          <input type="password" id="setDiscordToken" placeholder="Paste your Discord bot token here" />
          <div class="channel-card-actions">
            <a href="https://discord.com/developers/applications" target="_blank" rel="noopener">&#x2197; Discord Developer Portal</a>
            <button class="primary" onclick="saveDiscordToken()">Save & Enable</button>
          </div>
        </div>
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

<!-- Workspace panel -->
<div class="panel" id="workspacePanel">
  <div class="panel-header">
    <h3>Workspace</h3>
    <div class="panel-header-btns">
      <button id="wsBackBtn" class="hidden" onclick="wsBackToList()">Back</button>
      <button id="wsSaveBtn" class="hidden" onclick="wsSaveFile()">Save</button>
      <button onclick="fetchWorkspace()">Refresh</button>
      <button onclick="closeAllPanels()">Close</button>
    </div>
  </div>
  <div class="panel-content" id="workspaceContent">
    <div id="wsListView">
      <div id="wsBootstrapBanner" class="hidden">
        <div class="ws-bootstrap-banner">
          Onboarding not completed. <button onclick="wsResetBootstrap()">Reset Bootstrap</button>
        </div>
      </div>
      <div class="ws-file-list" id="wsFileList">
        <div class="panel-empty">Loading workspace files...</div>
      </div>
    </div>
    <div id="wsEditorView" class="hidden">
      <div class="ws-editor">
        <div class="ws-editor-header">Editing: <strong id="wsEditorFilename"></strong></div>
        <textarea id="wsEditorContent"></textarea>
      </div>
    </div>
  </div>
  <div class="panel-footer">
    <span id="wsFileCount">0 files</span>
    <span id="wsStatus"></span>
  </div>
</div>

<!-- Scheduler panel -->
<div class="panel" id="schedulerPanel">
  <div class="panel-header">
    <h3>Scheduler</h3>
    <div class="panel-header-btns">
      <button onclick="toggleCronForm()">+ New Job</button>
      <button onclick="fetchCronJobs()">Refresh</button>
      <button onclick="closeAllPanels()">Close</button>
    </div>
  </div>
  <div class="panel-content" id="schedulerContent">
    <div id="cronForm" class="hidden">
      <div class="cron-form">
        <h4>New Scheduled Job</h4>
        <div class="cron-form-row">
          <label>Name</label>
          <input type="text" id="cronName" placeholder="e.g., Daily Summary" />
        </div>
        <div class="cron-form-row">
          <label>Schedule Type</label>
          <select id="cronType">
            <option value="cron">Cron Expression</option>
            <option value="interval">Interval</option>
            <option value="once">One-time</option>
          </select>
        </div>
        <div class="cron-form-row">
          <label>Schedule Expression</label>
          <input type="text" id="cronExpr" placeholder="0 9 * * * (cron) or 30m (interval) or 2025-01-01T09:00:00Z (once)" />
        </div>
        <div class="cron-form-row">
          <label>Timezone (optional)</label>
          <input type="text" id="cronTimezone" placeholder="America/Chicago" />
        </div>
        <div class="cron-form-row">
          <label>Message (prompt sent to agent)</label>
          <textarea id="cronMessage" placeholder="What should the agent do when this job runs?"></textarea>
        </div>
        <div class="cron-form-actions">
          <button class="btn-primary" onclick="createCronJob()">Create Job</button>
          <button class="btn-secondary" onclick="toggleCronForm()">Cancel</button>
        </div>
      </div>
    </div>
    <div id="cronJobList">
      <div class="panel-empty">Loading scheduled jobs...</div>
    </div>
  </div>
  <div class="panel-footer">
    <span id="cronJobCount">0 jobs</span>
    <span></span>
  </div>
</div>

<!-- Skills panel -->
<div class="panel" id="skillsPanel">
  <div class="panel-header">
    <h3>Skills</h3>
    <div class="panel-header-btns">
      <button onclick="fetchSkills()">Refresh</button>
      <button onclick="closeAllPanels()">Close</button>
    </div>
  </div>
  <div class="panel-content" id="skillsContent">
    <div class="skill-tabs">
      <span class="skill-tab active" data-tab="installed" onclick="switchSkillTab('installed')">Installed</span>
      <span class="skill-tab" data-tab="browse" onclick="switchSkillTab('browse')">Browse</span>
    </div>
    <div id="skillsInstalledView">
      <div id="skillsList">
        <div class="panel-empty">Loading skills...</div>
      </div>
    </div>
    <div id="skillsBrowseView" class="hidden">
      <div class="skill-install-form">
        <input type="text" id="skillSearchInput" placeholder="Search ClawHub or enter GitHub owner/repo..." />
        <button onclick="searchSkills()">Search</button>
      </div>
      <div id="skillSearchResults"></div>
    </div>
  </div>
  <div class="panel-footer">
    <span id="skillCount">0 skills</span>
    <span></span>
  </div>
</div>

<!-- Toast container -->
<div class="toast-container" id="toastContainer"></div>

<!-- Health panel -->
<div class="panel" id="healthPanel">
  <div class="panel-header">
    <h3>System Health</h3>
    <div class="panel-header-btns">
      <button onclick="fetchHealth()">Refresh</button>
      <button onclick="closeAllPanels()">Close</button>
    </div>
  </div>
  <div class="panel-content" id="healthContent">
    <div class="panel-empty">Loading health data...</div>
  </div>
  <div class="panel-footer">
    <span id="healthTimestamp">Last checked: never</span>
    <div class="auto-refresh">
      <input type="checkbox" id="healthAutoRefresh" checked />
      <label for="healthAutoRefresh" style="cursor:pointer;">Auto-refresh (10s)</label>
    </div>
  </div>
</div>

<!-- Tasks panel -->
<div class="panel" id="tasksPanel">
  <div class="panel-header">
    <h3>Tasks</h3>
    <div class="panel-header-btns">
      <button onclick="fetchTasks()">Refresh</button>
      <button onclick="closeAllPanels()">Close</button>
    </div>
  </div>
  <div class="panel-content" id="tasksContent">
    <div class="task-add-bar">
      <input type="text" id="taskInput" placeholder="Add a task... (Enter to save)" />
      <select id="taskPrioritySelect" style="background:var(--bg3);border:1px solid var(--border2);border-radius:6px;padding:6px 8px;color:var(--text);font-size:12px;outline:none;cursor:pointer;">
        <option value="medium">Medium</option>
        <option value="high">High</option>
        <option value="low">Low</option>
      </select>
      <button onclick="addTask()">Add</button>
    </div>
    <div class="task-filters" id="taskFilters">
      <span class="task-filter active" data-filter="active" onclick="setTaskFilter(this)">Active</span>
      <span class="task-filter" data-filter="all" onclick="setTaskFilter(this)">All</span>
      <span class="task-filter" data-filter="done" onclick="setTaskFilter(this)">Done</span>
      <span class="task-filter" data-filter="high" onclick="setTaskFilter(this, 'priority')">High Priority</span>
    </div>
    <div id="taskList">
      <div class="task-empty">Loading tasks...</div>
    </div>
  </div>
  <div class="panel-footer">
    <span id="taskCount">0 tasks</span>
    <span id="taskDoneCount" style="color:var(--success);"></span>
  </div>
</div>

<!-- Notes panel -->
<div class="panel" id="notesPanel">
  <div class="panel-header">
    <h3>Notes</h3>
    <div class="panel-header-btns">
      <button id="noteBackBtn" class="hidden" onclick="noteBackToList()">Back</button>
      <button id="noteSaveBtn" class="hidden" onclick="saveCurrentNote()">Save</button>
      <button onclick="closeAllPanels()">Close</button>
    </div>
  </div>
  <div class="panel-content" id="notesContent">
    <div class="notes-layout">
      <div id="notesListView">
        <div class="notes-toolbar">
          <input type="text" id="noteSearchInput" placeholder="Search notes..." />
          <button onclick="createNewNote()">+ New</button>
        </div>
        <div id="notesList">
          <div class="note-empty">Loading notes...</div>
        </div>
      </div>
      <div id="noteEditorView" class="hidden">
        <div class="note-editor-header">
          <input type="text" id="noteNameInput" placeholder="Note name" />
        </div>
        <div class="note-editor-tabs">
          <span class="note-editor-tab active" data-tab="edit" onclick="switchNoteTab('edit')">Edit</span>
          <span class="note-editor-tab" data-tab="preview" onclick="switchNoteTab('preview')">Preview</span>
        </div>
        <div id="noteEditPane">
          <textarea class="note-textarea" id="noteContentInput" placeholder="Write your note in Markdown..."></textarea>
        </div>
        <div id="notePreviewPane" class="hidden">
          <div class="note-preview" id="notePreviewContent"></div>
        </div>
      </div>
    </div>
  </div>
  <div class="panel-footer">
    <span id="noteCount">0 notes</span>
    <span id="noteStatus"></span>
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
let healthInterval = null;
function openPanel(name) {
  closeAllPanels();
  currentPanel = name;
  const panel = $(name + 'Panel');
  panel.classList.add('open');
  backdrop.classList.add('open');
  if (name === 'logs') { fetchLogs(); startAutoRefresh(); }
  if (name === 'settings') { loadSettings(); checkVersion(); }
  if (name === 'health') { fetchHealth(); startHealthAutoRefresh(); }
  if (name === 'tasks') { fetchTasks(); }
  if (name === 'notes') { fetchNotes(); }
  if (name === 'workspace') { fetchWorkspace(); }
  if (name === 'scheduler') { fetchCronJobs(); }
  if (name === 'skills') { fetchSkills(); }
}

function closeAllPanels() {
  document.querySelectorAll('.panel').forEach(p => p.classList.remove('open'));
  backdrop.classList.remove('open');
  stopAutoRefresh();
  stopHealthAutoRefresh();
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
  chat.innerHTML = '<div class="welcome"><h2>Welcome to PennyClaw</h2><p>Your $0/month personal AI agent. Type a message to get started.</p><div class="shortcuts"><kbd>Ctrl</kbd>+<kbd>K</kbd> New chat &nbsp;<kbd>Ctrl</kbd>+<kbd>L</kbd> Clear &nbsp;<kbd>Ctrl</kbd>+<kbd>E</kbd> Export &nbsp;<kbd>Esc</kbd> Close panels<br><kbd>Ctrl</kbd>+<kbd>Shift</kbd>+<kbd>H</kbd> Health &nbsp;<kbd>Ctrl</kbd>+<kbd>Shift</kbd>+<kbd>T</kbd> Tasks &nbsp;<kbd>Ctrl</kbd>+<kbd>Shift</kbd>+<kbd>N</kbd> Notes &nbsp;<kbd>Ctrl</kbd>+<kbd>Shift</kbd>+<kbd>W</kbd> Workspace &nbsp;<kbd>Ctrl</kbd>+<kbd>Shift</kbd>+<kbd>J</kbd> Scheduler<br><kbd>Ctrl</kbd>+<kbd>S</kbd> Save note/file</div></div>';
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
    currentSettings = s;
    $('setProvider').value = s.provider || 'openai';
    $('setModel').value = s.model || '';
    $('setApiKey').value = '';
    $('setApiKey').placeholder = s.api_key || 'Enter API key';
    $('setBaseUrl').value = s.base_url || '';
    $('setMaxTokens').value = s.max_tokens || 4096;
    $('setTemperature').value = s.temperature != null ? s.temperature : 0.7;
    $('setSystemPrompt').value = s.system_prompt || '';
    loadChannelStatus();
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

// ===== Toast Notifications =====
function showToast(message, type) {
  type = type || 'info';
  const icons = { success: '\u2713', error: '\u2717', info: '\u2139' };
  const container = $('toastContainer');
  const toast = document.createElement('div');
  toast.className = 'toast ' + type;
  toast.innerHTML = '<span class="toast-icon">' + (icons[type] || '') + '</span>' + escapeHtml(message);
  container.appendChild(toast);
  setTimeout(() => { toast.style.opacity = '0'; toast.style.transition = 'opacity 0.3s'; setTimeout(() => toast.remove(), 300); }, 3000);
}

// ===== Health Dashboard =====
function startHealthAutoRefresh() {
  stopHealthAutoRefresh();
  if ($('healthAutoRefresh').checked) healthInterval = setInterval(fetchHealth, 10000);
}
function stopHealthAutoRefresh() {
  if (healthInterval) { clearInterval(healthInterval); healthInterval = null; }
}
$('healthAutoRefresh').addEventListener('change', () => {
  if (currentPanel === 'health') {
    if ($('healthAutoRefresh').checked) startHealthAutoRefresh();
    else stopHealthAutoRefresh();
  }
});

async function fetchHealth() {
  try {
    const res = await apiFetch('/api/health');
    const h = await res.json();
    renderHealth(h);
    updateHeaderHealth(h.status);
    $('healthTimestamp').textContent = 'Last checked: ' + new Date().toLocaleTimeString();
  } catch (e) {
    $('healthContent').innerHTML = '<div class="panel-empty">Failed to fetch health data.</div>';
    updateHeaderHealth('unknown');
  }
}

function updateHeaderHealth(status) {
  const el = $('headerStatus');
  if (status === 'healthy') {
    el.textContent = 'Healthy';
    el.style.color = 'var(--success)';
    el.style.setProperty('--dot-color', 'var(--success)');
  } else if (status === 'degraded') {
    el.textContent = 'Degraded';
    el.style.color = 'var(--warn)';
  } else if (status === 'unhealthy') {
    el.textContent = 'Unhealthy';
    el.style.color = 'var(--error)';
  } else {
    el.textContent = 'Online';
    el.style.color = 'var(--success)';
  }
}

function renderHealth(h) {
  const el = $('healthContent');
  const statusClass = h.status || 'healthy';
  const statusLabel = (h.status || 'healthy').charAt(0).toUpperCase() + (h.status || 'healthy').slice(1);
  const sys = h.system || {};
  const ag = h.agent || {};
  const checks = h.checks || {};

  const memPct = sys.memory_used_mb && sys.memory_total_mb ? Math.round(sys.memory_used_mb / sys.memory_total_mb * 100) : 0;
  const diskPct = sys.disk_used_gb && sys.disk_total_gb ? Math.round(sys.disk_used_gb / sys.disk_total_gb * 100) : 0;
  const memClass = memPct > 90 ? 'crit' : memPct > 75 ? 'warn' : 'ok';
  const diskClass = diskPct > 90 ? 'crit' : diskPct > 75 ? 'warn' : 'ok';

  let html = '<div class="health-status-banner ' + statusClass + '">';
  html += statusLabel + ' &mdash; ' + (h.version || 'unknown') + ' (' + (ag.provider || '?') + '/' + (ag.model || '?') + ')';
  html += '</div>';

  html += '<div class="health-grid">';
  html += '<div class="health-card"><div class="hc-label">Memory</div><div class="hc-value">' + (sys.memory_used_mb || 0).toFixed(0) + ' MB</div><div class="hc-sub">of ' + (sys.memory_total_mb || 0).toFixed(0) + ' MB</div><div class="gauge-bar"><div class="gauge-fill ' + memClass + '" style="width:' + memPct + '%"></div></div></div>';
  html += '<div class="health-card"><div class="hc-label">Disk</div><div class="hc-value">' + (sys.disk_used_gb || 0).toFixed(1) + ' GB</div><div class="hc-sub">of ' + (sys.disk_total_gb || 0).toFixed(1) + ' GB</div><div class="gauge-bar"><div class="gauge-fill ' + diskClass + '" style="width:' + diskPct + '%"></div></div></div>';
  html += '<div class="health-card"><div class="hc-label">Goroutines</div><div class="hc-value">' + (sys.goroutines || 0) + '</div><div class="hc-sub">active</div></div>';
  html += '<div class="health-card"><div class="hc-label">Uptime</div><div class="hc-value">' + formatUptime(sys.uptime_seconds) + '</div><div class="hc-sub">since start</div></div>';
  html += '</div>';

  html += '<div class="health-section"><h4>Agent Metrics</h4>';
  html += '<div class="health-row"><span class="hr-label">Total Requests</span><span class="hr-value">' + (ag.total_requests || 0) + '</span></div>';
  html += '<div class="health-row"><span class="hr-label">Total Errors</span><span class="hr-value" style="color:' + ((ag.total_errors || 0) > 0 ? 'var(--error)' : 'inherit') + '">' + (ag.total_errors || 0) + '</span></div>';
  html += '<div class="health-row"><span class="hr-label">Avg Latency</span><span class="hr-value">' + (ag.avg_latency_ms || 0).toFixed(0) + ' ms</span></div>';
  html += '<div class="health-row"><span class="hr-label">P99 Latency</span><span class="hr-value">' + (ag.p99_latency_ms || 0).toFixed(0) + ' ms</span></div>';
  html += '<div class="health-row"><span class="hr-label">Skills Loaded</span><span class="hr-value">' + (ag.skills_loaded || 0) + '</span></div>';
  html += '</div>';

  if (checks && Object.keys(checks).length) {
    html += '<div class="health-section"><h4>Health Checks</h4>';
    for (const [name, result] of Object.entries(checks)) {
      const icon = result === 'ok' ? '\u2713' : result === 'warn' ? '\u26A0' : '\u2717';
      const color = result === 'ok' ? 'var(--success)' : result === 'warn' ? 'var(--warn)' : 'var(--error)';
      html += '<div class="health-row"><span class="hr-label">' + escapeHtml(name) + '</span><span class="hr-value" style="color:' + color + '">' + icon + ' ' + result + '</span></div>';
    }
    html += '</div>';
  }

  el.innerHTML = html;
}

function formatUptime(seconds) {
  if (!seconds) return '0s';
  const d = Math.floor(seconds / 86400);
  const h = Math.floor((seconds % 86400) / 3600);
  const m = Math.floor((seconds % 3600) / 60);
  if (d > 0) return d + 'd ' + h + 'h';
  if (h > 0) return h + 'h ' + m + 'm';
  return m + 'm';
}

// Periodic header health check (every 30s)
setInterval(async () => {
  try {
    const res = await apiFetch('/api/health');
    const h = await res.json();
    updateHeaderHealth(h.status);
  } catch (e) {}
}, 30000);

// ===== Task Manager =====
let currentTaskFilter = 'active';
let currentTaskPriorityFilter = '';

$('taskInput').addEventListener('keydown', e => {
  if (e.key === 'Enter') { e.preventDefault(); addTask(); }
});

function setTaskFilter(el, type) {
  document.querySelectorAll('.task-filter').forEach(f => f.classList.remove('active'));
  el.classList.add('active');
  if (type === 'priority') {
    currentTaskFilter = '';
    currentTaskPriorityFilter = el.dataset.filter;
  } else {
    currentTaskFilter = el.dataset.filter;
    currentTaskPriorityFilter = '';
  }
  fetchTasks();
}

async function fetchTasks() {
  try {
    let url = '/api/tasks?';
    if (currentTaskFilter === 'active') url += 'status=active';
    else if (currentTaskFilter === 'done') url += 'status=done';
    else if (currentTaskFilter === 'all') url += 'status=all';
    if (currentTaskPriorityFilter) url += 'priority=' + currentTaskPriorityFilter + '&status=all';
    const res = await apiFetch(url);
    const data = await res.json();
    renderTasks(data.tasks || []);
  } catch (e) {
    $('taskList').innerHTML = '<div class="task-empty">Failed to load tasks.</div>';
  }
}

function renderTasks(tasks) {
  const el = $('taskList');
  if (!tasks.length) {
    el.innerHTML = '<div class="task-empty">No tasks found. Add one above!</div>';
    $('taskCount').textContent = '0 tasks';
    $('taskDoneCount').textContent = '';
    return;
  }
  const doneCount = tasks.filter(t => t.status === 'done').length;
  $('taskCount').textContent = tasks.length + ' task' + (tasks.length !== 1 ? 's' : '');
  $('taskDoneCount').textContent = doneCount > 0 ? doneCount + ' done' : '';
  // Update sidebar badge
  const activeCount = tasks.filter(t => t.status !== 'done').length;
  const badge = $('taskBadge');
  if (badge) badge.textContent = activeCount > 0 ? activeCount : '';

  el.innerHTML = tasks.map(t => {
    const isDone = t.status === 'done';
    const checkClass = isDone ? 'task-check checked' : 'task-check';
    const cardClass = isDone ? 'task-card done' : 'task-card';
    let meta = '';
    if (t.tags && t.tags.length) {
      meta += t.tags.map(tag => '<span class="task-tag">' + escapeHtml(tag) + '</span>').join('');
    }
    if (t.due_date) {
      meta += '<span class="task-due">Due: ' + escapeHtml(t.due_date) + '</span>';
    }
    return '<div class="' + cardClass + '" data-id="' + t.id + '">' +
      '<div class="task-card-header">' +
      '<div class="' + checkClass + '" onclick="toggleTask(' + t.id + ', ' + isDone + ')">' + (isDone ? '\u2713' : '') + '</div>' +
      '<span class="task-title">' + escapeHtml(t.title) + '</span>' +
      '<span class="task-priority ' + (t.priority || 'medium') + '">' + (t.priority || 'medium') + '</span>' +
      '<div class="task-actions"><button onclick="deleteTask(' + t.id + ')" title="Delete">&times;</button></div>' +
      '</div>' +
      (meta ? '<div class="task-meta">' + meta + '</div>' : '') +
      '</div>';
  }).join('');
}

async function addTask() {
  const title = $('taskInput').value.trim();
  if (!title) return;
  const priority = $('taskPrioritySelect').value;
  try {
    await apiFetch('/api/tasks', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ title, priority })
    });
    $('taskInput').value = '';
    showToast('Task added', 'success');
    fetchTasks();
  } catch (e) {
    showToast('Failed to add task', 'error');
  }
}

async function toggleTask(id, isDone) {
  try {
    await apiFetch('/api/tasks/' + id, {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ status: isDone ? 'todo' : 'done' })
    });
    fetchTasks();
  } catch (e) {
    showToast('Failed to update task', 'error');
  }
}

async function deleteTask(id) {
  if (!confirm('Delete this task?')) return;
  try {
    await apiFetch('/api/tasks/' + id, { method: 'DELETE' });
    showToast('Task deleted', 'success');
    fetchTasks();
  } catch (e) {
    showToast('Failed to delete task', 'error');
  }
}

// ===== Notes Manager =====
let currentNoteName = null;
let noteEditorDirty = false;

$('noteSearchInput').addEventListener('input', debounce(async function() {
  const q = $('noteSearchInput').value.trim();
  if (!q) { fetchNotes(); return; }
  try {
    const res = await apiFetch('/api/notes/search?q=' + encodeURIComponent(q));
    const data = await res.json();
    renderNotesList(data.notes || []);
  } catch (e) {}
}, 300));

function debounce(fn, delay) {
  let timer;
  return function() {
    clearTimeout(timer);
    const args = arguments;
    const ctx = this;
    timer = setTimeout(() => fn.apply(ctx, args), delay);
  };
}

async function fetchNotes() {
  try {
    const res = await apiFetch('/api/notes');
    const data = await res.json();
    renderNotesList(data.notes || []);
  } catch (e) {
    $('notesList').innerHTML = '<div class="note-empty">Failed to load notes.</div>';
  }
}

function renderNotesList(notes) {
  const el = $('notesList');
  $('noteCount').textContent = notes.length + ' note' + (notes.length !== 1 ? 's' : '');
  if (!notes.length) {
    el.innerHTML = '<div class="note-empty">No notes yet. Create one!</div>';
    return;
  }
  el.innerHTML = notes.map(n => {
    const active = n.name === currentNoteName ? ' active' : '';
    const updated = n.updated_at ? new Date(n.updated_at).toLocaleDateString() : '';
    const size = n.size ? formatBytes(n.size) : '';
    return '<div class="note-item' + active + '" onclick="openNote(\'' + escapeHtml(n.name).replace(/'/g, "\\'") + '\')">' +
      '<div class="note-item-info"><span class="note-item-name">' + escapeHtml(n.name) + '</span>' +
      '<span class="note-item-meta">' + updated + (size ? ' &middot; ' + size : '') + '</span></div>' +
      '<div class="note-item-actions"><button onclick="event.stopPropagation();deleteNote(\'' + escapeHtml(n.name).replace(/'/g, "\\'") + '\')" title="Delete">&times;</button></div>' +
      '</div>';
  }).join('');
}

function createNewNote() {
  currentNoteName = null;
  $('noteNameInput').value = '';
  $('noteContentInput').value = '';
  $('notesListView').classList.add('hidden');
  $('noteEditorView').classList.remove('hidden');
  $('noteBackBtn').classList.remove('hidden');
  $('noteSaveBtn').classList.remove('hidden');
  $('noteNameInput').focus();
  switchNoteTab('edit');
  noteEditorDirty = false;
}

async function openNote(name) {
  try {
    const res = await apiFetch('/api/notes/' + encodeURIComponent(name));
    const data = await res.json();
    currentNoteName = name;
    $('noteNameInput').value = name;
    $('noteContentInput').value = data.content || '';
    $('notesListView').classList.add('hidden');
    $('noteEditorView').classList.remove('hidden');
    $('noteBackBtn').classList.remove('hidden');
    $('noteSaveBtn').classList.remove('hidden');
    switchNoteTab('edit');
    noteEditorDirty = false;
  } catch (e) {
    showToast('Failed to open note', 'error');
  }
}

function noteBackToList() {
  if (noteEditorDirty && !confirm('Discard unsaved changes?')) return;
  $('notesListView').classList.remove('hidden');
  $('noteEditorView').classList.add('hidden');
  $('noteBackBtn').classList.add('hidden');
  $('noteSaveBtn').classList.add('hidden');
  currentNoteName = null;
  noteEditorDirty = false;
  fetchNotes();
}

async function saveCurrentNote() {
  const name = $('noteNameInput').value.trim();
  const content = $('noteContentInput').value;
  if (!name) { showToast('Note name is required', 'error'); return; }
  try {
    if (currentNoteName && currentNoteName !== name) {
      // Name changed: save as new, delete old
      await apiFetch('/api/notes', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ name, content })
      });
      await apiFetch('/api/notes/' + encodeURIComponent(currentNoteName), { method: 'DELETE' });
    } else if (currentNoteName) {
      await apiFetch('/api/notes/' + encodeURIComponent(name), {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ content })
      });
    } else {
      await apiFetch('/api/notes', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ name, content })
      });
    }
    currentNoteName = name;
    noteEditorDirty = false;
    showToast('Note saved', 'success');
    $('noteStatus').textContent = 'Saved';
    setTimeout(() => $('noteStatus').textContent = '', 2000);
  } catch (e) {
    showToast('Failed to save note', 'error');
  }
}

async function deleteNote(name) {
  if (!confirm('Delete note "' + name + '"?')) return;
  try {
    await apiFetch('/api/notes/' + encodeURIComponent(name), { method: 'DELETE' });
    showToast('Note deleted', 'success');
    if (currentNoteName === name) noteBackToList();
    else fetchNotes();
  } catch (e) {
    showToast('Failed to delete note', 'error');
  }
}

function switchNoteTab(tab) {
  document.querySelectorAll('.note-editor-tab').forEach(t => t.classList.toggle('active', t.dataset.tab === tab));
  if (tab === 'edit') {
    $('noteEditPane').classList.remove('hidden');
    $('notePreviewPane').classList.add('hidden');
  } else {
    $('noteEditPane').classList.add('hidden');
    $('notePreviewPane').classList.remove('hidden');
    $('notePreviewContent').innerHTML = renderMarkdown($('noteContentInput').value || '*No content*');
  }
}

// Track dirty state
$('noteContentInput').addEventListener('input', () => { noteEditorDirty = true; });
$('noteNameInput').addEventListener('input', () => { noteEditorDirty = true; });

// ===== Channel Config Functions =====
function loadChannelStatus() {
  // Check Telegram
  if (currentSettings && currentSettings.telegram_enabled) {
    $('telegramStatus').textContent = 'Connected';
    $('telegramStatus').className = 'channel-card-status connected';
  }
  // Check Email
  if (currentSettings && currentSettings.email_enabled) {
    $('emailStatus').textContent = 'Configured';
    $('emailStatus').className = 'channel-card-status connected';
    if (currentSettings.smtp_host) $('setSmtpHost').value = currentSettings.smtp_host;
    if (currentSettings.smtp_port) $('setSmtpPort').value = currentSettings.smtp_port;
    if (currentSettings.smtp_user) $('setSmtpUser').value = currentSettings.smtp_user;
  }
  // Check Webhook
  if (currentSettings && currentSettings.webhook_enabled) {
    $('webhookStatus').textContent = 'Enabled';
    $('webhookStatus').className = 'channel-card-status connected';
  }
  // Check Discord
  if (currentSettings && currentSettings.discord_enabled) {
    $('discordStatus').textContent = 'Connected';
    $('discordStatus').className = 'channel-card-status connected';
  }
  // Set webhook URL
  const host = window.location.origin;
  $('webhookUrl').value = host + '/api/webhooks';
}

async function saveTelegramToken() {
  const token = $('setTelegramToken').value.trim();
  if (!token) { showToast('Please enter a Telegram bot token', 'error'); return; }
  try {
    await apiFetch('/api/settings', {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ telegram_token: token, telegram_enabled: true })
    });
    showToast('Telegram connected! Restart required to activate.', 'success');
    $('telegramStatus').textContent = 'Connected';
    $('telegramStatus').className = 'channel-card-status connected';
    $('setTelegramToken').value = '';
  } catch (e) {
    showToast('Failed to save Telegram config', 'error');
  }
}

async function saveEmailConfig() {
  const host = $('setSmtpHost').value.trim();
  const port = parseInt($('setSmtpPort').value) || 587;
  const user = $('setSmtpUser').value.trim();
  const pass = $('setSmtpPass').value.trim();
  if (!host || !user) { showToast('SMTP host and username are required', 'error'); return; }
  try {
    await apiFetch('/api/settings', {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ email_enabled: true, smtp_host: host, smtp_port: port, smtp_user: user, smtp_pass: pass })
    });
    showToast('Email configured! Restart required to activate.', 'success');
    $('emailStatus').textContent = 'Configured';
    $('emailStatus').className = 'channel-card-status connected';
  } catch (e) {
    showToast('Failed to save email config', 'error');
  }
}

async function testEmail() {
  showToast('Sending test email...', 'info');
  try {
    await apiFetch('/api/settings', {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ test_email: true })
    });
    showToast('Test email sent (check server logs for result)', 'success');
  } catch (e) {
    showToast('Test email failed', 'error');
  }
}

async function saveWebhookConfig() {
  const secret = $('setWebhookSecret').value.trim();
  try {
    await apiFetch('/api/settings', {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ webhook_enabled: true, webhook_secret: secret || undefined })
    });
    showToast('Webhook enabled! Restart required to activate.', 'success');
    $('webhookStatus').textContent = 'Enabled';
    $('webhookStatus').className = 'channel-card-status connected';
  } catch (e) {
    showToast('Failed to save webhook config', 'error');
  }
}

function copyWebhookUrl() {
  const url = $('webhookUrl').value;
  navigator.clipboard.writeText(url).then(() => {
    showToast('Webhook URL copied!', 'success');
  }).catch(() => {
    $('webhookUrl').select();
    document.execCommand('copy');
    showToast('Webhook URL copied!', 'success');
  });
}

async function testWebhook() {
  showToast('Sending test webhook...', 'info');
  try {
    await fetch($('webhookUrl').value, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ event: 'test', message: 'Test webhook from PennyClaw UI', timestamp: new Date().toISOString() })
    });
    showToast('Test webhook sent!', 'success');
  } catch (e) {
    showToast('Test webhook failed', 'error');
  }
}

async function saveDiscordToken() {
  const token = $('setDiscordToken').value.trim();
  if (!token) { showToast('Please enter a Discord bot token', 'error'); return; }
  try {
    await apiFetch('/api/settings', {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ discord_token: token, discord_enabled: true })
    });
    showToast('Discord connected! Restart required to activate.', 'success');
    $('discordStatus').textContent = 'Connected';
    $('discordStatus').className = 'channel-card-status connected';
    $('setDiscordToken').value = '';
  } catch (e) {
    showToast('Failed to save Discord config', 'error');
  }
}

let currentSettings = null;

// ===== Health Indicator in Header =====
let healthPollInterval = null;

function startHealthPoll() {
  updateHeaderHealth();
  healthPollInterval = setInterval(updateHeaderHealth, 30000); // every 30s
}

async function updateHeaderHealth() {
  try {
    const res = await apiFetch('/api/health');
    const data = await res.json();
    const el = $('headerStatus');
    if (data.status === 'healthy') {
      el.textContent = '\u25CF Online';
      el.style.color = 'var(--success)';
    } else if (data.status === 'degraded') {
      el.textContent = '\u25CF Degraded';
      el.style.color = 'var(--warn)';
    } else {
      el.textContent = '\u25CF Unhealthy';
      el.style.color = 'var(--error)';
    }
  } catch (e) {
    const el = $('headerStatus');
    el.textContent = '\u25CF Offline';
    el.style.color = 'var(--error)';
  }
}

// Start health polling on page load
startHealthPoll();

// ===== Workspace Manager =====
let currentWsFile = null;
let wsEditorDirty = false;

async function fetchWorkspace() {
  try {
    const res = await apiFetch('/api/workspace');
    const data = await res.json();
    renderWorkspaceFiles(data.files || []);
    if (data.needs_bootstrap) {
      $('wsBootstrapBanner').classList.remove('hidden');
    } else {
      $('wsBootstrapBanner').classList.add('hidden');
    }
  } catch (e) {
    $('wsFileList').innerHTML = '<div class="panel-empty">Failed to load workspace files.</div>';
  }
}

function renderWorkspaceFiles(files) {
  const el = $('wsFileList');
  $('wsFileCount').textContent = files.length + ' file' + (files.length !== 1 ? 's' : '');
  if (!files.length) {
    el.innerHTML = '<div class="panel-empty">No workspace files yet.</div>';
    return;
  }
  el.innerHTML = files.map(f => {
    const active = f === currentWsFile ? ' active' : '';
    return '<div class="ws-file' + active + '" onclick="wsOpenFile(\'' + escapeHtml(f).replace(/'/g, "\\'") + '\')">' +
      '<span class="ws-file-name">' + escapeHtml(f) + '</span>' +
      '<div class="ws-file-actions"><button onclick="event.stopPropagation();wsDeleteFile(\'' + escapeHtml(f).replace(/'/g, "\\'") + '\')" title="Delete">&times;</button></div>' +
      '</div>';
  }).join('');
}

async function wsOpenFile(name) {
  try {
    const res = await apiFetch('/api/workspace/' + encodeURIComponent(name));
    const data = await res.json();
    currentWsFile = name;
    $('wsEditorFilename').textContent = name;
    $('wsEditorContent').value = data.content || '';
    $('wsListView').classList.add('hidden');
    $('wsEditorView').classList.remove('hidden');
    $('wsBackBtn').classList.remove('hidden');
    $('wsSaveBtn').classList.remove('hidden');
    wsEditorDirty = false;
  } catch (e) {
    showToast('Failed to open file', 'error');
  }
}

function wsBackToList() {
  if (wsEditorDirty && !confirm('Discard unsaved changes?')) return;
  $('wsListView').classList.remove('hidden');
  $('wsEditorView').classList.add('hidden');
  $('wsBackBtn').classList.add('hidden');
  $('wsSaveBtn').classList.add('hidden');
  currentWsFile = null;
  wsEditorDirty = false;
  fetchWorkspace();
}

async function wsSaveFile() {
  if (!currentWsFile) return;
  try {
    await apiFetch('/api/workspace/' + encodeURIComponent(currentWsFile), {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ content: $('wsEditorContent').value })
    });
    wsEditorDirty = false;
    showToast('File saved', 'success');
    $('wsStatus').textContent = 'Saved';
    setTimeout(() => $('wsStatus').textContent = '', 2000);
  } catch (e) {
    showToast('Failed to save file', 'error');
  }
}

async function wsDeleteFile(name) {
  if (!confirm('Delete workspace file "' + name + '"?')) return;
  try {
    await apiFetch('/api/workspace/' + encodeURIComponent(name), { method: 'DELETE' });
    showToast('File deleted', 'success');
    if (currentWsFile === name) wsBackToList();
    else fetchWorkspace();
  } catch (e) {
    showToast('Failed to delete file', 'error');
  }
}

async function wsResetBootstrap() {
  if (!confirm('Reset onboarding? The agent will re-run bootstrap on the next conversation.')) return;
  try {
    await apiFetch('/api/workspace/bootstrap', { method: 'POST' });
    showToast('Bootstrap reset', 'success');
    $('wsBootstrapBanner').classList.add('hidden');
  } catch (e) {
    showToast('Failed to reset bootstrap', 'error');
  }
}

$('wsEditorContent').addEventListener('input', () => { wsEditorDirty = true; });

// ===== Scheduler (Cron) =====
let cronJobs = [];

function toggleCronForm() {
  $('cronForm').classList.toggle('hidden');
}

async function fetchCronJobs() {
  try {
    const res = await apiFetch('/api/cron');
    const data = await res.json();
    cronJobs = data.jobs || [];
    renderCronJobs(cronJobs);
  } catch (e) {
    $('cronJobList').innerHTML = '<div class="panel-empty">Failed to load scheduled jobs.</div>';
  }
}

function renderCronJobs(jobs) {
  const el = $('cronJobList');
  $('cronJobCount').textContent = jobs.length + ' job' + (jobs.length !== 1 ? 's' : '');
  if (!jobs.length) {
    el.innerHTML = '<div class="panel-empty">No scheduled jobs yet. Create one!</div>';
    return;
  }
  el.innerHTML = jobs.map(j => {
    const badgeClass = j.enabled ? 'enabled' : 'disabled';
    const badgeText = j.enabled ? 'Enabled' : 'Disabled';
    const nextRun = j.next_run_at ? new Date(j.next_run_at).toLocaleString() : 'N/A';
    const lastRun = j.last_run_at ? new Date(j.last_run_at).toLocaleString() : 'Never';
    return '<div class="cron-job-card">' +
      '<div class="cron-job-header">' +
      '<span class="cron-job-name">' + escapeHtml(j.name) + '</span>' +
      '<span class="cron-job-badge ' + badgeClass + '">' + badgeText + '</span>' +
      '</div>' +
      '<div class="cron-job-schedule">' + escapeHtml(j.schedule_type) + ': ' + escapeHtml(j.schedule_expr) + (j.timezone ? ' (' + escapeHtml(j.timezone) + ')' : '') + '</div>' +
      '<div class="cron-job-message">' + escapeHtml(j.message || '') + '</div>' +
      '<div class="cron-job-meta">' +
      '<span>Next: ' + nextRun + '</span>' +
      '<span>Last: ' + lastRun + '</span>' +
      '</div>' +
      '<div class="cron-job-actions" style="margin-top:8px;">' +
      '<button onclick="runCronJob(' + j.id + ')" title="Run now">&#x25B6; Run</button>' +
      '<button onclick="toggleCronJob(' + j.id + ', ' + j.enabled + ')">' + (j.enabled ? 'Disable' : 'Enable') + '</button>' +
      '<button onclick="viewCronRuns(' + j.id + ')" title="View run history">History</button>' +
      '<button class="danger" onclick="deleteCronJob(' + j.id + ')" title="Delete">&times; Delete</button>' +
      '</div>' +
      '<div id="cronRuns' + j.id + '" class="cron-runs hidden"></div>' +
      '</div>';
  }).join('');
}

async function createCronJob() {
  const name = $('cronName').value.trim();
  const scheduleType = $('cronType').value;
  const scheduleExpr = $('cronExpr').value.trim();
  const timezone = $('cronTimezone').value.trim();
  const message = $('cronMessage').value.trim();
  if (!name || !scheduleExpr || !message) {
    showToast('Name, schedule, and message are required', 'error');
    return;
  }
  try {
    await apiFetch('/api/cron', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        name,
        schedule_type: scheduleType,
        schedule_expr: scheduleExpr,
        timezone: timezone || undefined,
        message,
        delete_after_run: scheduleType === 'once'
      })
    });
    showToast('Job created', 'success');
    $('cronName').value = '';
    $('cronExpr').value = '';
    $('cronTimezone').value = '';
    $('cronMessage').value = '';
    toggleCronForm();
    fetchCronJobs();
  } catch (e) {
    showToast('Failed to create job', 'error');
  }
}

async function runCronJob(id) {
  try {
    await apiFetch('/api/cron/' + id + '/run', { method: 'POST' });
    showToast('Job triggered', 'success');
    setTimeout(fetchCronJobs, 2000);
  } catch (e) {
    showToast('Failed to trigger job', 'error');
  }
}

async function toggleCronJob(id, currentlyEnabled) {
  try {
    await apiFetch('/api/cron/' + id, {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ enabled: !currentlyEnabled })
    });
    fetchCronJobs();
  } catch (e) {
    showToast('Failed to update job', 'error');
  }
}

async function deleteCronJob(id) {
  if (!confirm('Delete this scheduled job?')) return;
  try {
    await apiFetch('/api/cron/' + id, { method: 'DELETE' });
    showToast('Job deleted', 'success');
    fetchCronJobs();
  } catch (e) {
    showToast('Failed to delete job', 'error');
  }
}

async function viewCronRuns(jobId) {
  const el = $('cronRuns' + jobId);
  if (!el.classList.contains('hidden')) {
    el.classList.add('hidden');
    return;
  }
  try {
    const res = await apiFetch('/api/cron/' + jobId + '/runs');
    const data = await res.json();
    const runs = data.runs || [];
    if (!runs.length) {
      el.innerHTML = '<div style="font-size:11px;color:var(--text4);padding:6px 0;">No runs yet.</div>';
    } else {
      el.innerHTML = runs.map(r => {
        const started = new Date(r.started_at).toLocaleString();
        return '<div class="cron-run">' +
          '<span class="cron-run-status ' + (r.status || '') + '">' + (r.status || '?') + '</span>' +
          '<span class="cron-run-time">' + started + '</span>' +
          '<span class="cron-run-result">' + escapeHtml((r.result || '').substring(0, 100)) + '</span>' +
          '</div>';
      }).join('');
    }
    el.classList.remove('hidden');
  } catch (e) {
    showToast('Failed to load run history', 'error');
  }
}

// ===== Skills Manager =====
let installedSkills = [];

function switchSkillTab(tab) {
  document.querySelectorAll('.skill-tab').forEach(t => t.classList.toggle('active', t.dataset.tab === tab));
  if (tab === 'installed') {
    $('skillsInstalledView').classList.remove('hidden');
    $('skillsBrowseView').classList.add('hidden');
  } else {
    $('skillsInstalledView').classList.add('hidden');
    $('skillsBrowseView').classList.remove('hidden');
  }
}

async function fetchSkills() {
  try {
    const res = await apiFetch('/api/skills');
    const data = await res.json();
    installedSkills = data.skills || [];
    renderSkills(installedSkills);
  } catch (e) {
    $('skillsList').innerHTML = '<div class="panel-empty">Failed to load skills.</div>';
  }
}

function renderSkills(skills) {
  const el = $('skillsList');
  $('skillCount').textContent = skills.length + ' skill' + (skills.length !== 1 ? 's' : '');
  if (!skills.length) {
    el.innerHTML = '<div class="panel-empty">No skills installed.</div>';
    return;
  }
  el.innerHTML = skills.map(s => {
    const badgeClass = s.bundled ? 'bundled' : 'installed';
    const badgeText = s.bundled ? 'Bundled' : 'Installed';
    return '<div class="skill-card">' +
      '<div class="skill-card-header">' +
      '<span class="skill-name">' + escapeHtml(s.name) + '</span>' +
      '<span class="skill-version">' + escapeHtml(s.version || '') + '</span>' +
      '<span class="skill-badge ' + badgeClass + '">' + badgeText + '</span>' +
      '<div class="skill-actions">' +
      '<label class="skill-toggle"><input type="checkbox" ' + (s.enabled ? 'checked' : '') + ' onchange="toggleSkill(\'' + escapeHtml(s.name).replace(/'/g, "\\'") + '\', this.checked)" /><span class="slider"></span></label>' +
      (!s.bundled ? '<button style="background:none;border:none;color:var(--text4);cursor:pointer;font-size:14px;" onclick="uninstallSkill(\'' + escapeHtml(s.name).replace(/'/g, "\\'") + '\')" title="Uninstall">&times;</button>' : '') +
      '</div>' +
      '</div>' +
      '<div class="skill-desc">' + escapeHtml(s.description || '') + '</div>' +
      (s.author ? '<div class="skill-author">by ' + escapeHtml(s.author) + '</div>' : '') +
      '</div>';
  }).join('');
}

async function toggleSkill(name, enabled) {
  try {
    await apiFetch('/api/skills/' + encodeURIComponent(name), {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ enabled })
    });
    showToast('Skill ' + (enabled ? 'enabled' : 'disabled'), 'success');
  } catch (e) {
    showToast('Failed to update skill', 'error');
    fetchSkills();
  }
}

async function uninstallSkill(name) {
  if (!confirm('Uninstall skill "' + name + '"?')) return;
  try {
    await apiFetch('/api/skills/' + encodeURIComponent(name), { method: 'DELETE' });
    showToast('Skill uninstalled', 'success');
    fetchSkills();
  } catch (e) {
    showToast('Failed to uninstall skill', 'error');
  }
}

$('skillSearchInput').addEventListener('keydown', e => {
  if (e.key === 'Enter') { e.preventDefault(); searchSkills(); }
});

async function searchSkills() {
  const q = $('skillSearchInput').value.trim();
  if (!q) return;
  $('skillSearchResults').innerHTML = '<div class="panel-empty">Searching...</div>';
  try {
    // Try ClawHub search first
    const res = await apiFetch('/api/skills/search?q=' + encodeURIComponent(q));
    const data = await res.json();
    const results = data.skills || [];
    if (!results.length) {
      // If query looks like owner/repo, offer direct install
      if (q.includes('/')) {
        $('skillSearchResults').innerHTML = '<div class="panel-empty">No results on ClawHub.<br><button class="btn-primary" style="margin-top:8px;" onclick="installSkill(\'github\', \'' + escapeHtml(q) + '\')">Install from GitHub: ' + escapeHtml(q) + '</button></div>';
      } else {
        $('skillSearchResults').innerHTML = '<div class="panel-empty">No skills found for "' + escapeHtml(q) + '".</div>';
      }
      return;
    }
    $('skillSearchResults').innerHTML = results.map(s => {
      return '<div class="skill-card">' +
        '<div class="skill-card-header">' +
        '<span class="skill-name">' + escapeHtml(s.name || s.Name || '') + '</span>' +
        '<span class="skill-version">' + escapeHtml(s.version || s.Version || '') + '</span>' +
        '<button class="btn-primary" style="padding:3px 10px;font-size:11px;" onclick="installSkill(\'clawhub\', \'' + escapeHtml(s.name || s.Name || '') + '\')">Install</button>' +
        '</div>' +
        '<div class="skill-desc">' + escapeHtml(s.description || s.Description || '') + '</div>' +
        '</div>';
    }).join('');
  } catch (e) {
    $('skillSearchResults').innerHTML = '<div class="panel-empty">Search failed. Try again.</div>';
  }
}

async function installSkill(source, identifier) {
  try {
    const res = await apiFetch('/api/skills/install', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ source, identifier })
    });
    const data = await res.json();
    showToast('Installed: ' + (data.skill ? data.skill.name : identifier), 'success');
    fetchSkills();
    switchSkillTab('installed');
  } catch (e) {
    showToast('Installation failed', 'error');
  }
}

// ===== Keyboard shortcuts (extended) =====
document.addEventListener('keydown', e => {
  // Ctrl+Shift+H: Health panel
  if ((e.ctrlKey || e.metaKey) && e.shiftKey && e.key === 'H') { e.preventDefault(); openPanel('health'); }
  // Ctrl+Shift+T: Tasks panel
  if ((e.ctrlKey || e.metaKey) && e.shiftKey && e.key === 'T') { e.preventDefault(); openPanel('tasks'); }
  // Ctrl+Shift+N: Notes panel
  if ((e.ctrlKey || e.metaKey) && e.shiftKey && e.key === 'N') { e.preventDefault(); openPanel('notes'); }
  // Ctrl+Shift+W: Workspace panel
  if ((e.ctrlKey || e.metaKey) && e.shiftKey && e.key === 'W') { e.preventDefault(); openPanel('workspace'); }
  // Ctrl+Shift+J: Scheduler panel
  if ((e.ctrlKey || e.metaKey) && e.shiftKey && e.key === 'J') { e.preventDefault(); openPanel('scheduler'); }
  // Ctrl+S: Save note (when in note editor) or workspace file (when in ws editor)
  if ((e.ctrlKey || e.metaKey) && e.key === 's' && !e.shiftKey) {
    if (currentPanel === 'notes' && !$('noteEditorView').classList.contains('hidden')) {
      e.preventDefault(); saveCurrentNote();
    } else if (currentPanel === 'workspace' && !$('wsEditorView').classList.contains('hidden')) {
      e.preventDefault(); wsSaveFile();
    }
  }
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
