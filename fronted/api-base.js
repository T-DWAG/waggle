// API_BASE：留空表示同源，由 serve.py 把 /api/* 反代到后端
// （对应 blog_build 里"线上走 nginx 反代"的做法，好处是不用后端开 CORS）
window.API_BASE = '';
window.API_TARGET = 'http://127.0.0.1:8888';
