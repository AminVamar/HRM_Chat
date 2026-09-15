CREATE TABLE IF NOT EXISTS users (
    id BIGSERIAL PRIMARY KEY,
    username VARCHAR(255) NOT NULL UNIQUE,
    email VARCHAR(255) NOT NULL UNIQUE,
    avatar_url VARCHAR(512) NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS chats (
    id BIGSERIAL PRIMARY KEY,
    name VARCHAR(255) NOT NULL DEFAULT '',
    type VARCHAR(50) NOT NULL CHECK (type IN ('direct', 'group')),
    avatar_url VARCHAR(512) NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS chat_members (
    chat_id BIGINT NOT NULL REFERENCES chats(id) ON DELETE CASCADE,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role VARCHAR(50) NOT NULL CHECK (role IN ('owner', 'admin', 'member')) DEFAULT 'member',
    joined_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_read_message_id BIGINT NOT NULL DEFAULT 0,
    PRIMARY KEY (chat_id, user_id)
);

CREATE TABLE IF NOT EXISTS files (
    id BIGSERIAL PRIMARY KEY,
    filename VARCHAR(255) NOT NULL,
    file_path VARCHAR(512) NOT NULL,
    file_size BIGINT NOT NULL,
    mime_type VARCHAR(128) NOT NULL,
    uploader_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS messages (
    id BIGSERIAL PRIMARY KEY,
    chat_id BIGINT NOT NULL REFERENCES chats(id) ON DELETE CASCADE,
    sender_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    content TEXT NOT NULL DEFAULT '',
    file_id BIGINT REFERENCES files(id) ON DELETE SET NULL,
    is_edited BOOLEAN NOT NULL DEFAULT FALSE,
    is_deleted BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS reactions (
    id BIGSERIAL PRIMARY KEY,
    message_id BIGINT NOT NULL REFERENCES messages(id) ON DELETE CASCADE,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    reaction VARCHAR(64) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT unique_user_message_reaction UNIQUE (message_id, user_id)
);

CREATE INDEX IF NOT EXISTS idx_chat_members_user_id ON chat_members(user_id);
CREATE INDEX IF NOT EXISTS idx_messages_chat_id ON messages(chat_id);
CREATE INDEX IF NOT EXISTS idx_reactions_message_id ON reactions(message_id);

ALTER TABLE users ADD COLUMN IF NOT EXISTS avatar_url VARCHAR(512) NOT NULL DEFAULT '';
ALTER TABLE chats ADD COLUMN IF NOT EXISTS avatar_url VARCHAR(512) NOT NULL DEFAULT '';

INSERT INTO users (username, email) VALUES
('amin', 'amin@chat.local'),
('fed', 'fed@chat.local'),
('ernest', 'ernest@chat.local')
ON CONFLICT (username) DO NOTHING;

ALTER TABLE chats ADD COLUMN IF NOT EXISTS is_global BOOLEAN NOT NULL DEFAULT FALSE;
CREATE UNIQUE INDEX IF NOT EXISTS idx_chats_single_global ON chats (is_global) WHERE is_global = true;
INSERT INTO chats (name, type, is_global)
SELECT 'Общий чат', 'group', true
WHERE NOT EXISTS (SELECT 1 FROM chats WHERE is_global = true);
CREATE INDEX IF NOT EXISTS idx_messages_created_at ON messages(created_at);

-- direct_key = "<меньшийUserID>_<большийUserID>" для личных чатов,
-- чтобы для одной пары нельзя было создать два чата. Для групп — NULL.
ALTER TABLE chats ADD COLUMN IF NOT EXISTS direct_key TEXT;
CREATE UNIQUE INDEX IF NOT EXISTS idx_chats_direct_key ON chats (direct_key) WHERE direct_key IS NOT NULL;

-- Заполняем direct_key для старых чатов. Если у пары уже есть дубли,
-- ключ получает только самый старый чат, остальные остаются NULL.
UPDATE chats c SET direct_key = pair.key
FROM (
    SELECT MIN(keyed.chat_id) AS chat_id, keyed.key
    FROM (
        SELECT cm.chat_id,
               MIN(cm.user_id) || '_' || MAX(cm.user_id) AS key
        FROM chat_members cm
        JOIN chats ch ON ch.id = cm.chat_id
        WHERE ch.type = 'direct'
        GROUP BY cm.chat_id
        HAVING COUNT(*) = 2
    ) keyed
    GROUP BY keyed.key
) pair
WHERE c.id = pair.chat_id AND c.direct_key IS NULL;

-- Для пагинации сообщений (ORDER BY id DESC) и подсчёта непрочитанных.
CREATE INDEX IF NOT EXISTS idx_messages_chat_id_id ON messages(chat_id, id DESC);
