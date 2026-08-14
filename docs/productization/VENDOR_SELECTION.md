# First Commercial Adapter Vendor Dossier

Status: `VENDOR_DECISION_REQUIRED`

Evidence reviewed: 2026-08-14. This dossier compares documented integration
surfaces; it does not claim Goflow compatibility, vendor approval, customer
validation, or permission to process production data.

## Decision gate

No vendor-specific production adapter may start until all five conditions are
recorded: user approval of the vendor, sufficient official API documentation,
authorized sandbox/test credentials, confirmed legal/terms permission for the
use case, and non-production test data. None of the three candidates currently
meets the complete gate in this repository.

## Evidence summary

| Criterion | KiotViet | Sapo | Haravan |
| --- | --- | --- | --- |
| API availability | Retail Public API documents products, orders, invoices, customers, purchase orders, inventory-bearing product data, and webhooks. | Public REST documentation covers products and orders; webhook topics include order/product lifecycle events. | Omni Admin API documents orders, products, inventory scopes, transactions, and webhooks. |
| Daily reporting | Invoices/orders and inventory-bearing products could support a derived DailyOps view; no reviewed official aggregate endpoint proves the exact required metrics. | Orders and variant inventory could support a derived view; the exact reporting contract and product edition remain unproven. | Orders, transactions and inventory scopes could support a derived view; the exact aggregate DailyOps metrics remain unproven. |
| Authentication | OAuth 2.0 client credentials; API calls also require the retailer identity header. | Public apps use OAuth with requested scopes. Private Apps use API key/secret Basic Authentication. | Public apps use OAuth; private apps use a store-created Bearer token. |
| Test environment | No sandbox or developer-store workflow was found in the reviewed official retail API documentation. Requires vendor confirmation. | Partner app creation is documented, but no isolated sandbox/test-store guarantee was found in the reviewed official pages. Requires vendor confirmation. | Official Partner documentation explicitly provides development stores for app testing. |
| Pagination | List APIs document `pageSize` (commonly maximum 100) and `currentItem`; response includes total/page size. | Order lists document `limit` (maximum 250), `page`, and `since_id`. | Orders document list pagination with a maximum `limit` of 50. |
| Rate limits | Official retail API states GET is limited to 5,000 requests/hour. Retry headers and write limits need confirmation. | No numeric API rate policy was found in the reviewed official documentation. Requires confirmation before design. | Official policy documents leaky bucket 80, leak rate 4/second, HTTP 429, call-limit header, and `Retry-After`. |
| Webhooks | Documented registration and HMAC-SHA256 verification; HTTPS is required for confidentiality. | Documented topics include order/product/customer events. Delivery authentication/retry details require deeper official confirmation. | Documented HTTPS subscription, Bearer registration, HMAC-SHA256 verification, timeout and retry behavior. |
| Partner/approval path | Store-admin API enablement is documented; broader partner distribution, approval, branding, and commercial terms require written confirmation. | Partner dashboard/developer program is documented; store/public distribution review and commercial terms require written confirmation. | Partner account, Dev Shop, public/private app types, scopes, and App Store path are documented; current developer terms and review requirements still require acceptance/review. |
| Data privacy | Store/customer/order data is sensitive; the general privacy/terms page is not sufficient evidence of API processor terms. | Store/customer/order data is sensitive; API-specific processing and retention terms were not established by reviewed pages. | Scoped merchant authorization is documented; API-specific processing, retention and cross-system obligations still require terms review. |
| Lock-in/support risk | Retailer header, vendor schemas and client-credential lifecycle create vendor-specific code; support and version SLA are unconfirmed. | Product-edition, auth and schema differences create the highest discovery risk; support/version SLA is unconfirmed. | OAuth/scopes, pagination and schemas remain vendor-specific despite clearer docs; support/version SLA is unconfirmed. |
| Pilot readiness | Plausible business fit for a Vietnamese retail DailyOps pilot, but blocked on an authorized test store, terms, and selected pilot users. | Plausible, but blocked on product/API edition clarification, sandbox, rate policy, terms, and selected pilot users. | Strongest documented technical test path, but blocked on user vendor choice, terms acceptance, authorized Dev Shop credentials, and test data. |

## Candidate notes

### KiotViet

The current official [Retail Public API](https://www.kiotviet.vn/huong-dan-su-dung-kiotviet/retail-ket-noi-api/public-api/)
documents OAuth client credentials, the required `Retailer` and Bearer headers,
5,000 GET requests/hour, product/order/invoice/customer resources, offset-style
pagination, and webhook registration with HMAC-SHA256. The same page exposes
inventory quantities through product data and purchase-order resources. The
official [privacy and terms page](https://www.kiotviet.vn/chinh-sach-bao-mat-va-dieu-khoan-su-dung/)
places responsibility on users to protect store access and addresses personal
data generally, but it is not a substitute for API/partner terms review.

Fit: strong candidate only if the first 3-5 pilot stores actually use KiotViet.
The present generic adapter cannot acquire/refresh client-credential tokens or
inject both the retailer header and Bearer token, and it cannot map nested vendor
responses into DailyOps. A reviewed vendor adapter would therefore be required.

Open questions: test tenant availability, permitted retention and downstream
processing, write and burst limits, token rotation/revocation, endpoint version
policy, partner approval, support SLA, branding, and commercial terms.

### Sapo

Official Sapo documentation describes [OAuth for public apps](https://support.sapo.vn/oauth),
including app credentials, merchant authorization and scopes. It separately
documents [Private Apps](https://help.sapo.vn/ung-dung-rieng-private-apps) using
API key/secret Basic Authentication. The official order list documents
[`limit`, `page`, and `since_id`](https://support.sapo.vn/phuong-thuc-get-cua-order-phan-1),
while the [webhook reference](https://support.sapo.vn/sapo-webhook) lists order,
product and customer topics. Product variants expose inventory quantity in the
official [Product Variant reference](https://support.sapo.vn/product-variant).
The [developer overview](https://support.sapo.vn/gioi-thieu-api) and partner
portal establish an app ecosystem, but the reviewed public pages do not resolve
the exact sandbox, rate-limit, review, branding, privacy-processing, or
commercial policy for this pilot.

Fit: potentially broad commerce coverage, with higher discovery uncertainty.
The generic adapter deliberately rejects credential-bearing URLs and does not
implement Basic or OAuth installation, so it is not a Sapo compatibility claim.

Open questions: which Sapo product/API generation pilot stores use, test-store
provisioning, numeric rate/retry policy, inventory/report endpoints for that
edition, webhook verification/retry contract, data terms, app review, support,
and API lifecycle policy.

### Haravan

Official documentation provides a concrete [Partner and Dev Shop workflow](https://docs.haravan.com/docs/tutorials/build-an-app/),
including non-production data and app installation. Public apps use OAuth and
private apps can use [store-created Bearer tokens](https://docs.haravan.com/docs/tutorials/authentication/private-app-authentication/).
The [scope table](https://docs.haravan.com/docs/omni-apis/access-scopes/)
documents order, product and inventory permissions. The [Order API](https://docs.haravan.com/docs/omni-apis/orders/)
documents list/detail operations and a maximum list limit of 50. The official
[rate policy](https://docs.haravan.com/docs/omni-apis/api-call-limit/) specifies
an 80-request bucket, 4 requests/second leak rate, 429 behavior and retry
headers. [Webhook documentation](https://docs.haravan.com/docs/tutorials/webhooks/connect-webhook/)
requires HTTPS, describes HMAC-SHA256 verification, and documents retries.
Haravan also states that APIs are subject to authentication, rate limits, and
[developer terms](https://docs.haravan.com/docs/omni-apis/).

Fit: best documented path for a technical contract pilot because a Dev Shop is
explicit. That does not establish product-market fit among Goflow's intended
users or legal permission for this specific use case.

Open questions: pilot-store demand, approved data scopes, current developer
terms, app review/branding, privacy and retention obligations, support path,
token lifecycle, and whether private-app or public OAuth distribution is
appropriate.

## Privacy and architecture constraints

Orders and customer resources can contain names, contact details, addresses,
IP/device data, payment status, and other personal or commercially sensitive
data. Any approved adapter must request read-only least-privilege scopes,
normalize only DailyOps fields, discard vendor extras, keep credentials in the
vault, avoid payload logging, define retention/deletion, and test with synthetic
data. A privacy/DPA and vendor-terms review is mandatory before production data.

The Checkpoint I adapter remains a reference contract, not a direct connector:
GET-only, bounded response/pagination/retry, vault auth, closed errors, and
contract-projected output. OAuth installation/refresh, vendor pagination and
mapping, webhooks, and vendor-specific error semantics require separately
reviewed implementation.

## Effort estimate and conditional recommendation

These are engineering estimates, not vendor commitments, and begin only after
the decision gate is complete:

| Candidate | Discovery and sandbox proof | Read-only DailyOps adapter plus contract/E2E evidence |
| --- | --- | --- |
| KiotViet | 3-5 engineering days | 2-4 weeks |
| Sapo | 5-8 engineering days | 3-5 weeks |
| Haravan | 2-4 engineering days | 2-4 weeks |

Conditional recommendation:

1. Choose the vendor already used by at least two consenting target pilot
   stores; actual pilot demand outweighs documentation convenience.
2. If demand is tied or still unknown, run an access/terms spike with Haravan
   first because its official Dev Shop, scopes, rate limits and webhook behavior
   are the clearest of the three.
3. If KiotViet is the dominant pilot vendor, select it only after an authorized
   non-production tenant and written API/partner terms are confirmed.
4. Do not select Sapo until the target product edition, sandbox and numeric rate
   policy are resolved in writing.

Current decision: `VENDOR_DECISION_REQUIRED`. Continue beta work using only the
generic reference adapter and loopback normalized source. Do not request or
store vendor credentials in chat, source control, fixtures, or CI.
