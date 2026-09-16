CREATE TABLE users (
    -- Id: uuid 类型，主键，默认生成随机 UUID
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    -- Username: 唯一，非空
    username text NOT NULL,
    -- Password: 文本
    password text,
    -- Avatar: 文本
    avatar text,
    -- Status: smallint
    status smallint DEFAULT 3,
    -- LastLoginTime: 带时区的时间戳
    last_login_time timestamptz,
    -- CurrentPlan: 默认为 'free'
    current_plan varchar(20) DEFAULT 'free',
    -- Email: 变长字符，唯一，非空
    email varchar(100) NOT NULL,
    -- EmailVerified: 布尔值，默认为 false
    email_verified boolean DEFAULT false,
    -- 约束条件
    CONSTRAINT uni_users_username UNIQUE (username),
    CONSTRAINT uni_users_email UNIQUE (email)
);