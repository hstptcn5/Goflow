# DailyOps Source JSON Contract

DailyOps performs one HTTP `GET` against an absolute `http` or `https` URL.
The response must be JSON with these required fields:

| Field | Type | Rule |
| --- | --- | --- |
| `report_date` | string | Non-empty date or timestamp label. |
| `timezone` | string | Non-empty source timezone label. |
| `revenue` | number | JSON number, not a quoted currency string. |
| `order_count` | integer | Zero or greater. |
| `cancelled_refunded_count` | integer | Zero or greater. |
| `low_stock_summary` | string | Concise, already normalized summary. |
| `comparison_summary` | string | Concise prior-period comparison. |

Example using synthetic data:

```json
{
  "report_date": "2026-08-09",
  "timezone": "Asia/Bangkok",
  "revenue": 48250.75,
  "order_count": 314,
  "cancelled_refunded_count": 7,
  "low_stock_summary": "3 SKUs below threshold",
  "comparison_summary": "Revenue up 12.4% vs prior day"
}
```

The endpoint must return HTTP 2xx and valid JSON. Extra fields are ignored by
the report contract. The pilot endpoint must use sanitized, non-production data;
do not put credentials in the URL or response. Use **Test source** before
completing setup. An HTML page, relative URL, unsupported scheme, missing field,
wrong type, or malformed JSON is rejected without sending a Telegram message.

