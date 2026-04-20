BEGIN;

INSERT INTO energy_tariff_windows (
  window_id,
  site_id,
  tariff_code,
  price_usd_per_mwh,
  effective_from,
  effective_to
)
VALUES
  ('tariff-cl01-base-demo', 'site-cl-01', 'stable_base', 84, NOW() - INTERVAL '12 hours', NOW() + INTERVAL '12 hours'),
  ('tariff-cl02-peak-demo', 'site-cl-02', 'peak_response', 162, NOW() - INTERVAL '12 hours', NOW() + INTERVAL '12 hours')
ON CONFLICT (window_id) DO UPDATE
SET site_id = EXCLUDED.site_id,
    tariff_code = EXCLUDED.tariff_code,
    price_usd_per_mwh = EXCLUDED.price_usd_per_mwh,
    effective_from = EXCLUDED.effective_from,
    effective_to = EXCLUDED.effective_to,
    updated_at = NOW();

COMMIT;
