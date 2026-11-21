package rag

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strings"

	"github.com/listen808/citebase/internal/brand"
)

// hostPortPattern 允许的 Host 形状：域名/IPv4 加可选端口。
// 不接受引号、尖括号、反斜杠、换行 —— 这四个字符是这个文件里所有插值点
// （JS 字符串字面量、iframe 的 src 属性）的逃逸位。
var hostPortPattern = regexp.MustCompile(`^[A-Za-z0-9]([A-Za-z0-9.\-]{0,253}[A-Za-z0-9])?(:\d{1,5})?$`)

// SafeBaseURL 校验由请求头推导出来的基址。
//
// 为什么必须校验：这三处 `var origin='%s'` / `base='%s'` / `<iframe src="%s/ai/widget">`
// 填的是 c.Request.Host，而 Host 是客户端说了算的头。
// 请求打上 `Host: x';alert(document.cookie);//` 就能把一段 JS 塞进
// 管理员打开的挂件页与任何嵌入挂件的主机页 —— 那是一条按设计存在的 XSS。
// 校验失败返回 false，调用方必须拒绝而不是猜一个。
func SafeBaseURL(raw string) (string, bool) {
	if raw == "" {
		return "", false
	}
	u, err := url.Parse(raw)
	if err != nil {
		return "", false
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return "", false
	}
	host := u.Hostname()
	if host == "" || !hostPortPattern.MatchString(u.Host) {
		return "", false
	}
	// 主机名形式的地址已经按正则收紧，IP 字面量再确认一遍形状。
	if ip := net.ParseIP(host); ip != nil && ip.To4() == nil && !strings.Contains(host, ":") {
		return "", false
	}
	// 端口存在时必须是数字（正则已保证），这里只复核 URL 里没有 userinfo ——
	// `https://evil@good.example/` 会被当成 good.example 的凭据形式，
	// 而拼进 JS 字符串后 `@` 前后的内容仍属于同一个字面量。
	if u.User != nil {
		return "", false
	}
	if u.Path != "" && u.Path != "/" {
		return "", false
	}
	return u.Scheme + "://" + u.Host, true
}

// jsString 把值编码成 JS 字符串字面量（含引号）。
// 原先手工 ReplaceAll 转义反斜杠和单引号，但漏了行分隔符 U+2028/U+2029 ——
// 它们在 JS 源码里是换行，能直接把字符串字面量截断。JSON 编码器按 ECMAScript
// 附录处理，一并转义，且不需要我们维护转义表。
func jsString(s string) string {
	b, err := json.Marshal(s)
	if err != nil {
		return `""`
	}
	return string(b)
}

type EmbedWidget struct {
	BaseURL string
}

// NewEmbedWidget 建挂件。BaseURL 必须是已过 SafeBaseURL 的值；
// 传入未校验的串会在这里直接失败（返回 nil），调用方据此拒绝出页。
func NewEmbedWidget(baseURL string) *EmbedWidget {
	safe, ok := SafeBaseURL(baseURL)
	if !ok {
		return nil
	}
	return &EmbedWidget{BaseURL: safe}
}

func (ew *EmbedWidget) GenerateEmbedJS() string {
	return fmt.Sprintf(`(function(){
  var d=document,s=d.createElement('script');
  s.src=%s+'/ai/widget.js';
  s.async=true;
  d.head.appendChild(s);
})();`, jsString(ew.BaseURL))
}

func (ew *EmbedWidget) GenerateWidgetHTML() string {
	return fmt.Sprintf(`<div id="citebase-ai-widget" style="position:fixed;bottom:20px;right:20px;z-index:99999;">
  <div id="citebase-ai-btn" onclick="citebaseAI.toggle()" style="
    width:56px;height:56px;border-radius:28px;
    background:linear-gradient(135deg,#007AFF,#5856D6);
    box-shadow:0 4px 12px rgba(0,122,255,0.4);
    cursor:pointer;display:flex;align-items:center;justify-content:center;
    transition:transform 0.2s;">
    <svg width="28" height="28" viewBox="0 0 24 24" fill="none" stroke="white" stroke-width="2">
      <path d="M21 15a2 2 0 0 1-2 2H7l-4 4V5a2 2 0 0 1 2-2h14a2 2 0 0 1 2 2z"/>
    </svg>
  </div>
  <div id="citebase-ai-panel" style="
    display:none;position:absolute;bottom:70px;right:0;
    width:380px;height:520px;
    border-radius:16px;overflow:hidden;
    box-shadow:0 8px 32px rgba(0,0,0,0.15);
    background:white;border:1px solid #e5e5e5;">
    <iframe id="citebase-ai-frame" src="%s/ai/widget" style="width:100%%;height:100%%;border:none;"></iframe>
  </div>
</div>
<script>
var citebaseAI={
  open:false,
  toggle:function(){
    this.open=!this.open;
    document.getElementById('citebase-ai-panel').style.display=this.open?'block':'none';
  }
};
</script>`, ew.BaseURL)
}

func (ew *EmbedWidget) ServeWidgetPage(w http.ResponseWriter, r *http.Request) {
	html := fmt.Sprintf(`<!DOCTYPE html>
<html><head><meta charset="utf-8">
<meta name="viewport" content="width=device-width,initial-scale=1">
<title>%[1]s AI Assistant</title>
<style>
*{margin:0;padding:0;box-sizing:border-box}
body{font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',sans-serif;background:#f5f5f7;height:100vh;display:flex;flex-direction:column}
.header{background:linear-gradient(135deg,#007AFF,#5856D6);color:white;padding:16px;text-align:center;font-size:16px;font-weight:600}
.chat{flex:1;overflow-y:auto;padding:16px}
.msg{margin-bottom:12px;padding:10px 14px;border-radius:12px;max-width:85%%;word-wrap:break-word}
.msg.user{background:#007AFF;color:white;margin-left:auto;border-bottom-right-radius:4px}
.msg.ai{background:#e8e8ed;color:#1d1d1f;margin-right:auto;border-bottom-left-radius:4px}
.msg.ai a{color:#007AFF}
.input-area{padding:12px;background:white;border-top:1px solid #e5e5e5;display:flex;gap:8px}
.input-area input{flex:1;border:1px solid #d1d1d6;border-radius:8px;padding:10px;font-size:14px;outline:none}
.input-area input:focus{border-color:#007AFF}
.input-area button{background:#007AFF;color:white;border:none;border-radius:8px;padding:10px 20px;cursor:pointer;font-size:14px}
.loading{display:inline-block;width:8px;height:8px;border-radius:50%%;background:#007AFF;animation:pulse 1s infinite}
@keyframes pulse{0%%,100%%{opacity:1}50%%{opacity:0.3}}
</style></head><body>
<div class="header">%[1]s AI 知识助手</div>
<div class="chat" id="chat"></div>
<div class="input-area">
<input id="q" placeholder="输入问题..." onkeydown="if(event.key==='Enter')ask()">
<button onclick="ask()">发送</button>
</div>
<script>
var origin=%[2]s;
function ask(){
  var q=document.getElementById('q').value.trim();
  if(!q)return;
  document.getElementById('q').value='';
  addMsg('user',q);
  var chat=document.getElementById('chat');
  var ld=document.createElement('div');ld.className='msg ai';ld.innerHTML='<span class="loading"></span><span class="loading" style="animation-delay:0.2s"></span><span class="loading" style="animation-delay:0.4s"></span>';
  chat.appendChild(ld);chat.scrollTop=chat.scrollHeight;
  fetch(origin+'/api/v1/ai/stream/ask',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({question:q}),credentials:'include'})
  .then(function(r){return r.text()})
  .then(function(t){
    ld.remove();
    var lines=t.split('\\n');var content='';
    lines.forEach(function(l){if(l.startsWith('data: ')){var d=l.substring(6);if(d==='[DONE]')return;try{var j=JSON.parse(d);if(j.content)content+=j.content}catch(e){}}});
    addMsg('ai',content);
  }).catch(function(e){ld.remove();addMsg('ai','请求失败: '+e.message)});
}
function addMsg(t,c){var chat=document.getElementById('chat');var d=document.createElement('div');d.className='msg '+t;d.textContent=c;chat.appendChild(d);chat.scrollTop=chat.scrollHeight}
</script></body></html>`, brand.SiteName(), jsString(ew.BaseURL))

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("X-Frame-Options", "ALLOWALL")
	w.Header().Set("Content-Security-Policy", "frame-ancestors *")
	w.Write([]byte(html))
}

func (ew *EmbedWidget) ServeWidgetJS(w http.ResponseWriter, r *http.Request) {
	// 整段挂件 HTML 要塞进一个 JS 字符串字面量里，所以两个插值点走同一个编码器。
	// 原先手写的四条 ReplaceAll（反斜杠/单引号/换行/回车）漏了 U+2028 与 U+2029 ——
	// 那两个字符在 JS 源码里就是换行，能在这里把字符串截断，
	// 而它们可以合法地出现在被嵌的 HTML 内容里。jsString 用 JSON 编码器，
	// 这两个字符连同引号、控制字符一起处理，转义表不必由我们维护。
	js := fmt.Sprintf(`(function(){
  if(document.getElementById('citebase-ai-widget'))return;
  var base=%s;
  var div=document.createElement('div');
  div.id='citebase-ai-widget';
  div.innerHTML=%s;
  document.body.appendChild(div);
})();`, jsString(ew.BaseURL), jsString(ew.GenerateWidgetHTML()))

	w.Header().Set("Content-Type", "application/javascript")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Write([]byte(js))
}
