# fronted · agent-platform 前端联调台

给 03 用户管理那套接口做的测试界面。视觉沿用 `blog_build` 的像素 Mac 终端风
（黑底 + 终端绿 `#00ff41` + winbar + Fusion Pixel 字体 + 硬阴影 + 像素切角）。

纯静态 HTML/CSS/JS，没有构建步骤，没有依赖。

## 跑起来

两个终端：

```shell
# 1. 后端（必须在 app 目录下跑，配置在 app/etc/config.yml）
cd app
go run main.go

# 2. 前端
python fronted/serve.py            # 默认 5173
```

浏览器打开 <http://localhost:5173>。

> 端口 5173 是特意选的：后端邮箱验证成功后会 `302` 跳到 `http://localhost:5173/login`，
> 点邮件里的链接正好落回本页面。前端服务会把 `/login` 回落到 `index.html`。

## 为什么要有 serve.py

后端 `config.yml` 里写的是 `cors:`，但 thunder 读的字段是 **`cros`**
（`midd/cros.go` 里是 `conf.Cros`），所以 CORS 中间件实际没挂上，预检请求直接 404。
`serve.py` 把 `/api/*` **同源反代**到 `http://127.0.0.1:8888`，前端当同源用，绕开 CORS。
（这也正是 `blog_build/frontend/api-base.js` 里注释的"线上走 nginx 反代"做法。）

想改成直连后端也行：把 `api-base.js` 改成 `window.API_BASE = 'http://127.0.0.1:8888'`，
同时把 `app/etc/config.yml` 的 `cors:` 改成 `cros:` 并重启后端。

## 界面

| 视图 | 对应接口 | 说明 |
|---|---|---|
| `go /login` | `POST /api/v1/auth/login` | 用户名或邮箱登录，成功后进控制台 |
| `go /register` | `POST /api/v1/auth/register` | 注册后提示去邮箱点验证链接 |
| `go /forgot` | `/forgot-password` → `/verify-code` → `/reset-password` | 三步式，第二步拿到的 resetToken 自动填进第三步 |
| `go /console` | `POST /refresh-token`、`GET /subscription/current` | 展示 userInfo / token / 过期时间，可刷新 token |

页面底部有一块 **`/api` 请求日志终端**，每次请求的方法、路径、HTTP 状态、耗时和响应体都会
打在上面 —— 测后端主要看它。

响应码文案对齐 `common/biz/error.go`：1001 用户名已存在 / 1002 邮箱已存在 / 1003 密码格式错误 /
1004 无效 token / 1005 用户不存在 / 1006 邮箱未验证 / 1007 token 生成失败 / 999 db error。

## 已知的后端小问题（前端会原样展示）

- `userInfo.email` / `userInfo.avatar` 是空串 —— 后端 `token()` 只填了 id、username、status、role
- 注册时如果发信失败，整个注册事务回滚，接口返回 `999 db error`，用户不会入库
- 本地调试取验证码：`redis-cli --raw get "forgot_password_code:你的邮箱"`（本机 Redis 抢了 6379，别去容器里查）
