# DailyOps Schedule And Timezone

DailyOps supports either manual-only operation or one local daily schedule.
It does not accept a raw cron expression in the appliance UI.

- Leave **Daily schedule** off for manual-only operation.
- To schedule a report, choose a local `HH:MM` time and an IANA timezone such
  as `Asia/Bangkok`. The timezone, not the Windows display timezone name,
  determines the trigger instant.
- The dashboard shows whether scheduling is enabled, the timezone, and the next
  run. Recheck these values after setup, restart, and upgrade.
- A missed period is skipped; startup does not replay an old report.
- During a daylight-saving gap, the nonexistent local time is skipped. During
  a repeated hour, one deterministic instant is selected, never both.
- Manual and scheduled runs share the same concurrency guard. A second request
  while a report is running is rejected and does not send another message.
- Setup migration or required revalidation suspends delivery until the user
  reviews setup and completes it again.

After changing time or timezone, confirm the displayed next run. For pilot
acceptance, observe one scheduled report, restart, and then observe the next
scheduled report without a replay at startup.

