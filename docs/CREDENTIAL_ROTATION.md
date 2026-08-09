# Credential Rotation

Use this process when a bot token or other credential value changes.

1. Start the appliance with the existing data directory.
2. Open the setup/dashboard page.
3. Reopen setup if the pack is already complete.
4. Create or replace the credential for the affected slot.
5. Run the connection test when the slot supports one.
6. Complete setup again.
7. Run the workflow with non-production sample data and confirm the latest execution.

The credential slot file stores credential IDs and expected types only. Decrypted values remain in the encrypted credential store. Do not edit `pack-credentials.json` to paste secret values; that file is not a secret-value store.

If a credential is deleted or has the wrong type, appliance readiness returns to `NEEDS_SETUP` and names the logical slot key.
