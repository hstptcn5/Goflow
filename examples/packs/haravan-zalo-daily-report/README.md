# Haravan → Zalo Daily Report

Vietnam-market adapter preview. It demonstrates a secure, local-first orchestration path:

1. Goflow reads recent orders from an authorized Haravan Admin API endpoint using an encrypted bearer credential.
2. A local JavaScript node calculates a compact order-count/value summary.
3. Goflow builds a recipient/message JSON payload.
4. Goflow POSTs that payload to an **authorized Zalo OA delivery adapter endpoint** using a second encrypted bearer credential.

## Important boundary

This Pack does not bypass Haravan or Zalo permissions and does not hard-code an undocumented Zalo messaging endpoint. The Zalo delivery URL must be an endpoint/adapter you are authorized to use and must accept:

```json
{
  "recipient_id": "...",
  "message": "..."
}
```

with a bearer credential in the `Authorization` header.

The default Haravan URL reads recent orders; adjust supported query parameters to match the reporting window you need. For a production connector, validate your Haravan app scopes and your Zalo OA message entitlement before enabling a schedule.
