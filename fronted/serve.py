#!/usr/bin/env python3
"""fronted 本地联调服务器：静态文件 + /api 反向代理

用法:
    python serve.py [port]        # 默认 5173

为什么需要它:
  1. 后端 config.yml 写的是 cors:，而 thunder 读的是 cros（midd/cros.go 里是 conf.Cros），
     所以后端实际没有挂 CORS 中间件，跨域请求会被浏览器拦。
     这里把 /api/* 同源反代到 127.0.0.1:8888，前端当同源用，绕开 CORS。
  2. 默认端口 5173，正好是后端邮箱验证成功后 302 跳转的地址，
     点完邮件里的链接会直接落回本页面。
"""
import http.server
import os
import sys
import urllib.error
import urllib.request

PORT = int(sys.argv[1]) if len(sys.argv) > 1 else 5173
BACKEND = os.environ.get('BACKEND', 'http://127.0.0.1:8888')
ROOT = os.path.dirname(os.path.abspath(__file__))


class NoRedirect(urllib.request.HTTPRedirectHandler):
    """不自动跟随 302，原样透传给浏览器（邮箱验证要靠这个跳转）"""

    def redirect_request(self, req, fp, code, msg, headers, newurl):
        return None


_opener = urllib.request.build_opener(NoRedirect)


class Handler(http.server.SimpleHTTPRequestHandler):
    def _status_of(self, code):
        self._status_code = code

    def send_response(self, code, message=None):
        self._status_code = code
        super().send_response(code, message)

    def end_headers(self):
        if getattr(self, '_status_code', 200) == 200 and self.path.endswith(
            ('.woff2', '.woff', '.ttf', '.svg', '.png', '.jpg', '.webp', '.ico')
        ):
            self.send_header('Cache-Control', 'public, max-age=31536000, immutable')
        else:
            self.send_header('Cache-Control', 'no-cache')
        super().end_headers()

    # ---------- 反代 ----------
    def _proxy(self):
        length = int(self.headers.get('Content-Length') or 0)
        body = self.rfile.read(length) if length else None
        req = urllib.request.Request(BACKEND + self.path, data=body, method=self.command)
        for h in ('Content-Type', 'Authorization', 'Accept'):
            if self.headers.get(h):
                req.add_header(h, self.headers[h])

        try:
            with _opener.open(req) as r:
                status, headers, payload = r.status, r.headers, r.read()
        except urllib.error.HTTPError as e:
            status, headers, payload = e.code, e.headers, e.read()
        except Exception as e:  # 后端没起 / 连接被拒
            payload = ('{"code":"PROXY_ERR","msg":"后端不可达: %s"}' % e).encode('utf-8')
            status, headers = 502, {}

        self.send_response(status)
        self.send_header('Content-Type', headers.get('Content-Type') or 'application/json; charset=utf-8')
        location = headers.get('Location')
        if location:
            self.send_header('Location', location)
        self.send_header('Content-Length', str(len(payload)))
        self.end_headers()
        if payload:
            self.wfile.write(payload)

    def _is_api(self):
        return self.path == '/api' or self.path.startswith('/api/')

    def do_GET(self):
        if self._is_api():
            return self._proxy()
        # SPA 回落：无扩展名且文件不存在（如 /login）→ index.html
        path = self.path.split('?')[0]
        if not os.path.splitext(path)[1] and not os.path.exists(
            os.path.join(ROOT, path.lstrip('/'))
        ):
            self.path = '/index.html'
        return super().do_GET()

    def do_POST(self):
        if self._is_api():
            return self._proxy()
        self.send_error(405)

    def do_PUT(self):
        if self._is_api():
            return self._proxy()
        self.send_error(405)

    def do_DELETE(self):
        if self._is_api():
            return self._proxy()
        self.send_error(405)

    def do_PATCH(self):
        if self._is_api():
            return self._proxy()
        self.send_error(405)

    def do_OPTIONS(self):
        if self._is_api():
            return self._proxy()
        self.send_error(405)

    def log_message(self, fmt, *a):
        sys.stderr.write("[fronted] %s\n" % (fmt % a))


if __name__ == '__main__':
    os.chdir(ROOT)
    print("fronted  →  http://localhost:%d" % PORT)
    print("api 代理  →  %s/api/*" % BACKEND)
    http.server.HTTPServer(('0.0.0.0', PORT), Handler).serve_forever()
