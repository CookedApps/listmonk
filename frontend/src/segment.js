export const ATTRIB = 'attribs.';

export const CORE_OPS = {
  email: ['eq', 'neq', 'contains', 'not_contains', 'one_of'],
  name: ['eq', 'neq', 'contains', 'not_contains', 'one_of'],
  status: ['eq', 'neq', 'one_of'],
  created_at: ['in_last_days', 'after', 'before'],
  list: ['in', 'not_in'],
  campaign: ['received', 'not_received', 'opened', 'not_opened'],
};

export const ATTRIB_OPS = {
  text: ['eq', 'neq', 'contains', 'not_contains', 'one_of', 'is_set', 'not_set'],
  number: ['gte', 'lte', 'eq', 'neq', 'gt', 'lt', 'is_set', 'not_set'],
  bool: ['is_true', 'is_not_true'],
  date: ['in_last_days', 'not_in_last_days', 'after', 'before', 'is_set', 'not_set'],
};

export const NO_VALUE_OPS = ['is_set', 'not_set', 'is_true', 'is_not_true'];

const isRuleComplete = (r) => (r.rules ? r.rules.length > 0 && r.rules.every(isRuleComplete)
  : NO_VALUE_OPS.includes(r.op) || (r.value !== null && r.value !== undefined && r.value !== ''));

/** Reports whether every rule of a segment, including grouped ones, has the value its operator needs. */
export const isSegmentComplete = (seg) => !seg || seg.rules.every(isRuleComplete);
