# Review email notifications

Human-review requests create a durable PostgreSQL outbox entry in the same transaction as the review-state change. Both candidate source reviews and challenge scientific reviews are covered. Repeated writes of the same state do not create another email; entering human review again after leaving it creates a new request. This does not change which challenges require human approval.

The worker checks the queue every ten seconds, separately from verification work. It skips reviews that have been resolved, withdrawn, or adopted before sending. Existing actionable reviews are queued by migration 017. Messages contain the challenge title, review reason, and an editor sign-in link. Emails never approve or publish a challenge.

## Connect SendGrid

Set these private environment variables on the API/worker app:

- `SENDGRID_API_KEY`: a dedicated SendGrid key with **Mail Send only**.
- `REVIEW_EMAIL_FROM`: one plain email address covered by SendGrid sender verification or domain authentication.
- `REVIEW_EMAIL_TO`: one plain reviewer email address.
- `PUBLIC_ORIGIN`: the HTTPS website origin used for review links.

Use Fly secrets, never Git or public browser configuration. For example, prepare a permission-0600 file outside the repository containing the three mail settings, then import it through stdin with `fly secrets import --app science-ladder-api < /private/path/sendgrid.env`. Avoid credentials in shell arguments or logs. The sender can be changed later through `REVIEW_EMAIL_FROM`, without a code change. The website domain and sending domain can differ. Authenticate a new sending domain in SendGrid before switching to it.

Without valid configuration, review requests remain pending without consuming attempts. The editor's `/review` page shows configuration, recent email activity, a test-email button, and retry controls. Test emails are limited to one per hour and clearly labeled.

## Delivery and failures

Multiple workers claim rows with database locks and fenced leases. SendGrid HTTP 202 is recorded as **accepted**, with its message identifier. This does not assert inbox delivery: consult SendGrid Email Activity for delivery, bounce, or suppression details. No event webhook is installed in this version. Click and open tracking are disabled.

Explicit throttling/server failures retry with exponential backoff, up to eight attempts. Other HTTP failures stop and appear in the review console. Transport failures and expired in-flight leases become **uncertain**: SendGrid may already have received the message. Check its activity before manually retrying; the console requires acknowledging the possibility of a duplicate. There is no claim of exactly-once delivery across the network. Provider response bodies and credentials are never logged or stored as errors.

Recipient addresses come only from private configuration. Creator-controlled text is bounded plain text and cannot change the destination. Only editors/operators with browser sessions can queue tests or retries. Review notification delivery does not use the verification VPS.

References: [SendGrid Mail Send API](https://www.twilio.com/docs/sendgrid/api-reference/mail-send/mail-send), [API keys](https://www.twilio.com/docs/sendgrid/ui/account-and-settings/api-keys), [sender identity](https://www.twilio.com/docs/sendgrid/for-developers/sending-email/sender-identity).

The production website origin is `https://scienceladder.org`. Changing the website origin changes links in newly generated review emails. The sender identity is configured separately; website DNS does not itself authenticate a SendGrid sender.
