import { describe, expect, it } from 'vitest';
import { applianceMessages, optionLabel, packLocaleCopy } from '../src/components/applianceI18n';

describe('appliance internationalization contract', () => {
  it('ships English and Vietnamese UI dictionaries', () => {
    expect(applianceMessages.en.runNow).toBe('Run now');
    expect(applianceMessages.vi.runNow).toBe('Chạy ngay');
    expect(applianceMessages.vi.credentials).toBe('Thông tin xác thực');
  });

  it('localizes the three Vietnam launch packs without changing stable config values', () => {
    expect(packLocaleCopy['official.daily-business-report'].vi.name).toBe('Báo cáo kinh doanh mỗi ngày');
    expect(packLocaleCopy['official.low-stock-alert'].vi.name).toBe('Cảnh báo tồn kho');
    expect(packLocaleCopy['official.haravan-zalo-daily-report'].vi.name).toBe('Báo cáo Haravan → Zalo');
    expect(optionLabel('vi', 'output_language', 'vi')).toBe('Tiếng Việt');
    expect(optionLabel('vi', 'ai_provider', 'none')).toBe('Không dùng AI');
    expect(optionLabel('en', 'ai_provider', 'none')).toBe('none');
  });
});
