package domain

import "errors"

var (
	ErrNotFound        = errors.New("resource not found")
	ErrUserNotFound    = errors.New("user not found")
	ErrUnauthorized    = errors.New("unauthorized: user not found")
	ErrForbidden       = errors.New("forbidden: action not allowed")
	ErrInvalidChatType = errors.New("invalid chat type: must be 'direct' or 'group'")
	ErrCannotSetOwner  = errors.New("cannot assign owner role: owner is only the chat creator")
	ErrAlreadyExists   = errors.New("resource already exists")
	ErrInvalidInput    = errors.New("invalid input data")
	ErrUserNotInChat   = errors.New("user is not a member of this chat")
	ErrMessageDeleted  = errors.New("message has been deleted")

	ErrDirectChatSelf     = errors.New("cannot create direct chat with yourself")
	ErrDirectChatMemberOp = errors.New("member management operations are not allowed on direct chats")
	ErrDirectChatLeave    = errors.New("cannot leave a direct chat")
	ErrDirectChatRename   = errors.New("cannot rename a direct chat")
	ErrGroupMinMembers    = errors.New("group chat must have at least 3 members (including creator)")

	ErrGlobalChatOp = errors.New("this operation is not allowed on the global chat")
)
