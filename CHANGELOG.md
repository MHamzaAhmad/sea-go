# simpleemailapi-go

## 0.3.0

### Breaking Changes

- **Async-Only Email Delivery**: Removed `async` flag from send email API. All emails are now processed asynchronously by SimpleEmailAPI, which handles delivery, retries, and monitoring automatically.
- **Simplified Response**: The send email response no longer includes `message_id` since delivery is always async.

### Features

- **Email Threading**: Added `reply_to` field for easier email threading and conversation management. Use this field to reference the email you're replying to.

## 0.1.0

### Features

- Initial release
- `NewClient()` - Create typed Email API client
- `client.Send()` - Send transactional emails
- `client.OnReceive()` - Stream email events with typed callbacks
- `client.EmailService` - Full EmailService client access
- `client.DomainService` - Full DomainService client access
- All proto-generated types exported for full type safety
