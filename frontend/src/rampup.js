export function recommendedRampDays(total, start, growth) {
  if (total <= start || start < 1) {
    return 1;
  }
  if (growth <= 1) {
    return Math.ceil(total / start);
  }

  const ratio = total / start;
  return Math.ceil(Math.log(ratio * (growth - 1) + 1) / Math.log(growth) - 1e-9);
}

export function rampCaps(total, days, growth) {
  const n = Math.max(days, 1);
  const g = Math.max(growth, 1);

  let weightSum = 0;
  for (let i = 0; i < n; i += 1) {
    weightSum += g ** i;
  }

  const caps = [];
  let remaining = total;
  for (let i = 0; i < n - 1; i += 1) {
    const cap = Math.round((total * g ** i) / weightSum);
    caps.push(cap);
    remaining -= cap;
  }
  caps.push(Math.max(remaining, 0));

  return caps;
}
