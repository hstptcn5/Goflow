# DailyOps Beta Manual Acceptance Checklist

Use synthetic or approved non-production data and a dedicated pilot Telegram
bot. Record failures as failures; do not infer a pass from automated CI.

- [ ] User consent recorded before the pilot and before diagnostics sharing.
- [ ] Artifact name, exact commit, unsigned marker, and SHA-256 verified.
- [ ] ZIP extracted to an ordinary user-writable directory.
- [ ] SmartScreen outcome recorded without disabling security controls.
- [ ] Double-click launch serves local health, bootstrap, and UI.
- [ ] Source contract test passes with sanitized JSON.
- [ ] Telegram `getMe`/`getChat` test passes without sending a message.
- [ ] Manual-only or daily schedule/timezone selected intentionally.
- [ ] Setup completes without editing JSON or developer intervention.
- [ ] One **Run now** action produces exactly one expected report.
- [ ] Duplicate click/request produces no duplicate report.
- [ ] Success/error/history/next-run display is understandable.
- [ ] Restart retains setup, credential reference, schedule, and history and
  sends no unsolicited report.
- [ ] Offline update retains state; revalidation is completed if requested.
- [ ] One post-update manual report, scheduled report, and restart pass.
- [ ] Redacted diagnostics export reviewed and contains no private data.
- [ ] Backup, credential rotation, stop/restart, rollback, uninstall, and
  external data deletion steps are understood.
- [ ] Feedback template completed, including usefulness and willingness to
  continue/pay without treating either answer as acceptance evidence alone.

Commercial adapter go/no-go requires at least three consenting pilot users,
most completing setup without direct developer intervention, no duplicate
report blocker, no config/credential/schedule loss across restart or upgrade,
supportable diagnostics, confirmed recurring-task value, and a credible signal
of willingness to continue or pay. Until real pilot evidence satisfies every
criterion, the decision remains `MANUAL_GATE_REQUIRED`.

