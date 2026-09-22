package auths

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"mime"
	"net/smtp"
	"strings"
	"time"

	"common/biz"
	"model"

	"github.com/google/uuid"
	"github.com/mszlu521/thunder/cache"
	"github.com/mszlu521/thunder/config"
	"github.com/mszlu521/thunder/errs"
	"github.com/mszlu521/thunder/logs"
	"github.com/mszlu521/thunder/tools/jwt"
	"github.com/mszlu521/thunder/tools/randoms"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

type Service struct {
	repo Repository
}

func NewService() *Service {
	return &Service{
		repo: NewModel(),
	}
}

func (s *Service) register(req RegisterReq) (*RegisterResp, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// Check if username already exists
	u, err := s.repo.findByUserName(ctx, req.Username)
	if err != nil {
		logs.Errorf("find auths err:%v", err)
		return nil, errs.DBError
	}
	if u != nil {
		return nil, biz.ErrUserNameExisted
	}
	// Check if email already exists
	u, err = s.repo.findByEmail(ctx, req.Email)
	if err != nil {
		logs.Errorf("find email err:%v", err)
		return nil, errs.DBError
	}
	if u != nil {
		return nil, biz.ErrEmailExisted
	}

	password, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, biz.ErrPasswordFormat
	}

	// Generate verification token
	tokenBytes := make([]byte, 16)
	if _, err := rand.Read(tokenBytes); err != nil {
		return nil, errs.DBError
	}
	verifyToken := hex.EncodeToString(tokenBytes)
	userId := uuid.New()
	// Store the verification token in Redis with a 24-hour expiration
	redisCache := cache.NewRedisCache()
	tokenKey := fmt.Sprintf("verify_token:%s", verifyToken)
	logs.Infof("storing verify token in Redis: %s and userId: %s", tokenKey, userId)
	if err := redisCache.Set(tokenKey, userId.String(), 24*60*60); err != nil { // 24 hours
		logs.Errorf("failed to store verify token in Redis: %v", err)
		return nil, errs.DBError
	}

	u = &model.User{
		Id:            userId,
		Username:      req.Username,
		Password:      string(password),
		LastLoginTime: time.Now(),
		Status:        model.UserStatusPending, // Set status to pending until email verification
		Avatar:        "default",
		CurrentPlan:   model.FreePlan,
		Email:         req.Email,
		EmailVerified: false,
	}

	// 先完成用户落库。邮件属于外部依赖，不能放在数据库事务里，
	// 否则 SMTP 失败会导致注册回滚并统一返回 db error。
	if err = s.repo.transaction(func(tx *gorm.DB) error {
		if err := s.repo.saveUser(ctx, tx, u); err != nil {
			logs.Errorf("create user err:%v", err)
			return err
		}
		return nil
	}); err != nil {
		return nil, errs.DBError
	}

	// 本地或 SMTP 暂不可用时仍保留注册结果，避免用户因邮件服务故障无法注册。
	// 验证 token 已提前写入 Redis；邮件恢复后可重新补发验证邮件。
	message := "注册成功，请检查您的邮箱并点击验证链接完成注册"
	if err := s.sendVerificationEmail(u.Email, u.Username, verifyToken); err != nil {
		logs.Errorf("send verification email failed for %s: %v", u.Email, err)
		message = "注册成功，但验证邮件发送失败，请联系管理员或稍后重试"
	}
	return &RegisterResp{Message: message}, nil
}

// buildMail 组装邮件内容
// QQ 邮箱会校验 From 头，中文主题也必须做 RFC2047 编码，否则返回 550
func buildMail(from, to, subject, body string) []byte {
	headers := []string{
		"From: " + from,
		"To: " + to,
		"Subject: " + mime.QEncoding.Encode("utf-8", subject),
		"MIME-Version: 1.0",
		`Content-Type: text/plain; charset="UTF-8"`,
		"Content-Transfer-Encoding: 8bit",
	}
	return []byte(strings.Join(headers, "\r\n") + "\r\n\r\n" + body + "\r\n")
}

func (s *Service) sendVerificationEmail(email string, username string, token string) error {
	// Get email configuration from config
	emailConfig := config.GetConfig().Email

	// If email is not configured, skip sending
	if emailConfig.Host == nil || emailConfig.Port == nil {
		logs.Warn("Email not configured, skipping verification email")
		return nil
	}

	// Email content
	subject := "请验证您的邮箱地址"
	verifyURL := fmt.Sprintf("%s/api/v1/auth/verify-email?token=%s", emailConfig.GetBaseURL(), token)
	body := fmt.Sprintf("尊敬的 %s，\n\n感谢您注册我们的服务！\n\n请点击以下链接验证您的邮箱地址：\n%s\n\n如果链接无法点击，请复制并粘贴到浏览器地址栏中。\n\n谢谢！\n", username, verifyURL)

	// Set up authentication information
	auth := smtp.PlainAuth("", emailConfig.GetUsername(), emailConfig.GetPassword(), emailConfig.GetHost())

	// Connect to the server, authenticate, and send the email
	to := []string{email}
	msg := buildMail(emailConfig.GetFrom(), email, subject, body)

	addr := fmt.Sprintf("%s:%d", emailConfig.GetHost(), emailConfig.GetPort())
	err := smtp.SendMail(addr, auth, emailConfig.GetFrom(), to, msg)
	if err != nil {
		return fmt.Errorf("failed to send email: %w", err)
	}

	return nil
}

func (s *Service) verifyEmail(token string) (any, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// 从Redis中获取用户ID
	redisCache := cache.NewRedisCache()
	tokenKey := fmt.Sprintf("verify_token:%s", token)
	userIdStr, err := redisCache.Get(tokenKey)
	if err != nil || userIdStr == "" {
		return nil, biz.ErrInvalidToken
	}
	// 删除已使用的令牌
	redisCache.Set(tokenKey, "", 1) // 设置1秒后过期

	// 根据用户ID查找用户
	userId, err := uuid.Parse(userIdStr)
	if err != nil {
		logs.Errorf("failed to parse user ID from token: %v", err)
		return nil, biz.ErrInvalidToken
	}

	u, err := s.repo.findById(ctx, userId)
	if err != nil {
		logs.Errorf("find user by ID err:%v", err)
		return nil, errs.DBError
	}
	if u == nil {
		return nil, biz.ErrUserNotFound
	}
	if u.EmailVerified {
		// User already verified, just login
		return nil, nil
	}
	// Update user status and email verification
	u.EmailVerified = true
	u.Status = model.UserStatusNormal
	err = s.repo.transaction(func(tx *gorm.DB) error {
		if err := s.repo.updateUser(ctx, tx, u); err != nil {
			logs.Errorf("update user err:%v", err)
			return err
		}
		return nil
	})
	if err != nil {
		return nil, errs.DBError
	}
	return nil, nil
}

func (s *Service) login(loginReq LoginReq) (*LoginResp, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	u, err := s.repo.findByUserNameOrEmail(ctx, loginReq.Username)
	if err != nil {
		logs.Errorf("find auths err:%v", err)
		return nil, biz.ErrUserNotFound
	}
	if u == nil {
		return nil, biz.ErrUserNotFound
	}
	// Check if user has verified their email
	if !u.EmailVerified {
		return nil, biz.ErrEmailNotVerified
	}

	if err := bcrypt.CompareHashAndPassword([]byte(u.Password), []byte(loginReq.Password)); err != nil {
		return nil, biz.ErrPasswordFormat
	}
	return s.token(err, u)
}

func (s *Service) token(err error, u *model.User) (*LoginResp, error) {
	//生成token和refreshToken
	expire := config.GetConfig().Jwt.GetExpire()
	refreshExpire := config.GetConfig().Jwt.GetRefresh()
	token, err := jwt.GenToken(u.Id.String(), u.Username, expire)
	if err != nil {
		logs.Errorf("gen token err:%v", err)
		return nil, biz.ErrTokenGen
	}
	refreshToken, err := jwt.GenToken(u.Id.String(), u.Username, refreshExpire)
	if err != nil {
		logs.Errorf("gen refresh token err:%v", err)
		return nil, biz.ErrTokenGen
	}
	return &LoginResp{
		Expire:        time.Now().Add(expire).UnixMilli(),
		RefreshExpire: time.Now().Add(refreshExpire).UnixMilli(),
		RefreshToken:  refreshToken,
		Token:         token,
		UserInfo: model.UserDTO{
			Id:       u.Id,
			Status:   u.Status,
			Username: u.Username,
			Role:     "admin",
		},
	}, nil
}

func (s *Service) forgotPassword(req ForgotPasswordReq) error {
	// 检查邮箱是否存在
	u, err := s.repo.findByEmail(context.Background(), req.Email)
	if err != nil {
		logs.Errorf("find email err:%v", err)
		return errs.DBError
	}
	if u == nil {
		return nil
	}

	// 生成验证码
	code, err := randoms.Gen6Code()
	if err != nil {
		logs.Errorf("gen code err:%v", err)
		return errs.DBError
	}
	// 将验证码存储在Redis中，设置5分钟过期时间
	redisCache := cache.NewRedisCache()
	codeKey := fmt.Sprintf("forgot_password_code:%s", req.Email)
	if err := redisCache.Set(codeKey, code, 5*60); err != nil { // 5分钟
		logs.Errorf("failed to store forgot password code in Redis: %v", err)
		return errs.DBError
	}

	// 发送验证码邮件
	if err := s.sendForgotPasswordEmail(u.Email, u.Username, code); err != nil {
		logs.Errorf("send forgot password email err:%v", err)
		return errs.DBError
	}

	return nil
}

func (s *Service) sendForgotPasswordEmail(email, username, code string) error {
	// Get email configuration from config
	emailConfig := config.GetConfig().Email

	// If email is not configured, skip sending
	if emailConfig.Host == nil || emailConfig.Port == nil {
		logs.Warn("Email not configured, skipping forgot password email")
		return nil
	}

	// Email content
	subject := "您的验证码"
	body := fmt.Sprintf("尊敬的 %s，\n\n您正在重置密码，验证码是：%s\n\n验证码5分钟内有效，如非本人操作请忽略。\n\n谢谢！\n", username, code)

	// Set up authentication information
	auth := smtp.PlainAuth("", emailConfig.GetUsername(), emailConfig.GetPassword(), emailConfig.GetHost())

	// Connect to the server, authenticate, and send the email
	to := []string{email}
	msg := buildMail(emailConfig.GetFrom(), email, subject, body)

	addr := fmt.Sprintf("%s:%d", emailConfig.GetHost(), emailConfig.GetPort())
	err := smtp.SendMail(addr, auth, emailConfig.GetFrom(), to, msg)
	if err != nil {
		return fmt.Errorf("failed to send email: %w", err)
	}

	return nil
}

func (s *Service) verifyCode(req VerifyCodeReq) (string, error) {
	// 从Redis中获取验证码
	redisCache := cache.NewRedisCache()
	codeKey := fmt.Sprintf("forgot_password_code:%s", req.Email)
	storedCode, err := redisCache.Get(codeKey)
	if err != nil || storedCode == "" {
		return "", biz.ErrInvalidToken
	}

	// 验证验证码是否正确
	if storedCode != req.Code {
		return "", biz.ErrInvalidToken
	}

	// 验证码正确，生成一个用于重置密码的临时令牌
	resetToken, err := s.generateResetToken(req.Email)
	if err != nil {
		return "", err
	}

	// 删除已使用的验证码
	redisCache.Set(codeKey, "", 1) // 设置1秒后过期

	return resetToken, nil
}

// generateResetToken 为重置密码功能生成重置令牌
func (s *Service) generateResetToken(email string) (string, error) {
	// Generate reset token
	tokenBytes := make([]byte, 16)
	if _, err := rand.Read(tokenBytes); err != nil {
		return "", errs.DBError
	}
	resetToken := hex.EncodeToString(tokenBytes)

	// Store the reset token in Redis with a 1-hour expiration
	redisCache := cache.NewRedisCache()
	tokenKey := fmt.Sprintf("reset_token:%s", resetToken)
	if err := redisCache.Set(tokenKey, email, 60*60); err != nil { // 1 hour
		logs.Errorf("failed to store reset token in Redis: %v", err)
		return "", errs.DBError
	}

	return resetToken, nil
}

func (s *Service) resetPassword(req ResetPasswordReq) error {
	// 验证重置令牌
	email, err := s.validateResetToken(req.Token)
	if err != nil {
		return errs.NewError(400, "无效或已过期的验证码")
	}

	// 确保邮箱匹配
	if email != req.Email {
		return errs.NewError(400, "邮箱不匹配")
	}

	// 查找用户
	user, err := s.repo.findByEmail(context.Background(), email)
	if err != nil {
		return errs.NewError(500, "用户不存在")
	}

	// 生成新密码的哈希值
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		return errs.NewError(500, "密码处理失败")
	}

	// 更新密码
	user.Password = string(hashedPassword)
	err = s.repo.transaction(func(tx *gorm.DB) error {
		return s.repo.updateUser(context.Background(), tx, user)
	})
	if err != nil {
		return errs.NewError(500, "更新密码失败")
	}

	return nil
}

func (s *Service) validateResetToken(token string) (string, error) {
	// 从Redis中获取邮箱
	redisCache := cache.NewRedisCache()
	tokenKey := fmt.Sprintf("reset_token:%s", token)
	email, err := redisCache.Get(tokenKey)
	if err != nil || email == "" {
		return "", biz.ErrInvalidToken
	}

	// 删除已使用的令牌
	redisCache.Set(tokenKey, "", 1) // 设置1秒后过期

	return email, nil
}
