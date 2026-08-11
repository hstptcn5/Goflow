# Goflow Ecosystem Alpha Pilot Guide

This guide is for a small alpha pilot. It does not claim that interviews, customers, revenue, vendor access, or market validation already occurred.

For the portable DailyOps Windows appliance, use the [Windows Pilot Guide](WINDOWS_PILOT_GUIDE.md) and [Windows Pilot Feedback Template](WINDOWS_PILOT_FEEDBACK_TEMPLATE.md).

## Recruitment Checklist

- Recruit exactly three users who currently prepare recurring operational or sales reports.
- Confirm each user can use non-production sample data.
- Confirm each user can run an unsigned local development artifact on a test machine.
- Confirm no production secrets will be shared through chat, issues, screenshots, or committed files.
- Schedule an observed setup session and a follow-up reliability review.

## Discovery Questions

1. What recurring report do you prepare today, and who reads it?
2. Which systems or files provide the source data?
3. How long does the report take, and where do mistakes usually happen?
4. What would make the output trustworthy enough to use repeatedly?
5. What setup step would make you stop before finishing?

## Required Sample Data Contract

For the DailyOps pack, sample data must match:

```json
{
  "date": "2026-08-09",
  "sales_total": 1234.56,
  "orders": 42,
  "low_stock": [
    { "sku": "SKU-1", "name": "Sample item", "quantity": 2 }
  ]
}
```

Use sanitized data. Remove customer names, addresses, phone numbers, emails, payment data, private notes, and production identifiers unless the pilot explicitly requires them and consent is documented outside the repository.

## Consent And Privacy

- Explain that the artifact is unsigned alpha software.
- Record what sample data is used and who approved it.
- Do not request production credentials.
- Do not accept secrets through chat, GitHub issues, screenshots, or files committed to the repo.
- Delete local temp data after each test unless the user asks to keep it for follow-up.

## Setup Success Metrics

- Time from download to first local page load.
- Time from first page load to setup ready.
- Number of failed setup attempts.
- Whether connection test succeeds without support.
- Whether the user understands where data and credentials are stored.

## Reliability Metrics

- Successful workflow runs divided by attempted runs.
- Error count by source fetch, transform, credential, and delivery stage.
- Time to diagnose a failed run from the dashboard.
- Whether diagnostics can be shared without secrets.
- Whether rerun produces duplicate delivery in failure scenarios.

## Pricing Experiment

Pricing is an experiment, not a validated market fact. Ask what value the user would expect from removing the manual reporting task, and test at most one clearly labeled hypothetical price range after the workflow pain is confirmed.

Do not infer willingness to pay from polite feedback.

## Support And Rollback

- Keep the previous data directory backup before changing setup.
- To rollback, stop `pack run`, restore the previous data directory, and restart.
- If credentials are suspected exposed, rotate them in the upstream provider and then in Goflow.
- Record bugs as technical symptoms and reproduction steps; do not attach production data or secrets.
