<template>
  <div class="segment-builder">
    <div v-if="rules.length > 1" class="match is-size-7 mb-2">
      {{ $t('campaigns.segmentMatch') }}
      <b-select v-model="match" size="is-small" :disabled="disabled" @input="emit">
        <option value="all">{{ $t('campaigns.segmentMatchAll') }}</option>
        <option value="any">{{ $t('campaigns.segmentMatchAny') }}</option>
      </b-select>
    </div>

    <div v-for="r in rules" :key="r.id" class="rule columns is-variable is-1 is-multiline mb-0"
      data-cy="segment-rule">
      <div class="column is-narrow">
        <b-select :value="r.field" size="is-small" :disabled="disabled" @input="(f) => onFieldChange(r, f)"
          data-cy="segment-field">
          <optgroup :label="$t('campaigns.segmentFieldsSubscriber')">
            <option v-for="f in coreFields" :key="f" :value="f">{{ $t(`campaigns.segmentField.${f}`) }}</option>
          </optgroup>
          <optgroup v-if="attribFields.length" :label="$t('campaigns.segmentFieldsAttribs')">
            <option v-for="a in attribFields" :key="a.field" :value="a.field">{{ a.key }}</option>
          </optgroup>
        </b-select>
      </div>
      <div class="column is-narrow">
        <b-select v-model="r.op" size="is-small" :disabled="disabled" @input="emit" data-cy="segment-op">
          <option v-for="op in opsFor(r)" :key="op" :value="op">{{ $t(`campaigns.segmentOp.${op}`) }}</option>
        </b-select>
      </div>
      <div class="column">
        <template v-if="valueKind(r) === 'none'" />
        <b-select v-else-if="valueKind(r) === 'list'" v-model="r.value" expanded size="is-small" :disabled="disabled"
          @input="emit" data-cy="segment-value">
          <option v-for="l in lists" :key="l.id" :value="l.id">{{ l.name }}</option>
        </b-select>
        <b-select v-else-if="valueKind(r) === 'campaign'" v-model="r.value" expanded size="is-small"
          :disabled="disabled" @input="emit" data-cy="segment-value">
          <option v-for="c in campaigns" :key="c.id" :value="c.id">{{ c.name }}</option>
        </b-select>
        <b-select v-else-if="valueKind(r) === 'status'" v-model="r.value" expanded size="is-small"
          :disabled="disabled" @input="emit" data-cy="segment-value">
          <option v-for="s in statuses" :key="s" :value="s">{{ $t(`subscribers.status.${s}`) }}</option>
        </b-select>
        <b-field v-else-if="valueKind(r) === 'days'" class="days">
          <b-input v-model.number="r.value" type="number" min="0" size="is-small" :disabled="disabled" @input="emit"
            data-cy="segment-value" />
          <p class="control">
            <span class="button is-static is-small">
              {{ r.op === 'not_in_last_days' ? $t('campaigns.segmentDaysAgo') : $t('campaigns.segmentDays') }}
            </span>
          </p>
        </b-field>
        <b-input v-else-if="valueKind(r) === 'number'" v-model.number="r.value" type="number" step="any"
          size="is-small" :disabled="disabled" @input="emit" data-cy="segment-value" />
        <b-input v-else-if="valueKind(r) === 'date'" v-model="r.value" type="date" size="is-small" :disabled="disabled"
          @input="emit" data-cy="segment-value" />
        <b-input v-else v-model="r.value" size="is-small" :disabled="disabled" @input="emit"
          :placeholder="r.op === 'one_of' ? 'a, b, c' : ''" data-cy="segment-value" />
      </div>
      <div class="column is-narrow">
        <b-button size="is-small" icon-left="trash-can-outline" :disabled="disabled" @click="removeRule(r)"
          data-cy="segment-remove" />
      </div>
    </div>

    <p v-if="rules.length === 0" class="is-size-7 has-text-grey mb-2">
      {{ $t('campaigns.segmentNoRules') }}
    </p>

    <div class="buttons are-small">
      <b-button size="is-small" icon-left="plus" :disabled="disabled" @click="addRule('email', 'contains')"
        data-cy="segment-add">
        {{ $t('campaigns.segmentAddRule') }}
      </b-button>
      <b-button size="is-small" icon-left="account-check-outline" :disabled="disabled"
        @click="addRule('campaign', 'received')" data-cy="segment-same-as">
        {{ $t('campaigns.segmentSameAs') }}
      </b-button>
      <b-button size="is-small" icon-left="account-off-outline" :disabled="disabled"
        @click="addRule('campaign', 'not_received')" data-cy="segment-exclude">
        {{ $t('campaigns.segmentExclude') }}
      </b-button>
    </div>
  </div>
</template>

<script>
import Vue from 'vue';

const ATTRIB = 'attribs.';

const CORE_OPS = {
  email: ['eq', 'neq', 'contains', 'not_contains', 'one_of'],
  name: ['eq', 'neq', 'contains', 'not_contains', 'one_of'],
  status: ['eq', 'neq', 'one_of'],
  created_at: ['in_last_days', 'after', 'before'],
  list: ['in', 'not_in'],
  campaign: ['received', 'not_received', 'opened', 'not_opened'],
};

const ATTRIB_OPS = {
  text: ['eq', 'neq', 'contains', 'not_contains', 'one_of', 'is_set', 'not_set'],
  number: ['gte', 'lte', 'eq', 'neq', 'gt', 'lt', 'is_set', 'not_set'],
  bool: ['is_true', 'is_not_true'],
  date: ['in_last_days', 'not_in_last_days', 'after', 'before', 'is_set', 'not_set'],
};

const NO_VALUE_OPS = ['is_set', 'not_set', 'is_true', 'is_not_true'];

/** Reports whether every rule of a segment has the value its operator needs. */
export const isSegmentComplete = (seg) => !seg || seg.rules.every((r) => NO_VALUE_OPS.includes(r.op)
  || (r.value !== null && r.value !== ''));

let nextID = 1;

export default Vue.extend({
  name: 'SegmentBuilder',

  props: {
    value: { type: Object, default: null },
    lists: { type: Array, default: () => [] },
    campaigns: { type: Array, default: () => [] },
    attribKeys: { type: Array, default: () => [] },
    disabled: Boolean,
  },

  data() {
    return {
      coreFields: Object.keys(CORE_OPS),
      statuses: ['enabled', 'disabled', 'blocklisted'],
      match: 'all',
      rules: [],
      lastEmitted: null,
    };
  },

  computed: {
    attribFields() {
      const known = this.attribKeys.map((a) => ({ field: ATTRIB + a.key, key: a.key }));
      const used = this.rules
        .filter((r) => r.field.startsWith(ATTRIB) && !known.some((a) => a.field === r.field))
        .map((r) => ({ field: r.field, key: r.field.slice(ATTRIB.length) }));
      return [...known, ...used];
    },
  },

  methods: {
    isAttrib(r) {
      return r.field.startsWith(ATTRIB);
    },

    opsFor(r) {
      return this.isAttrib(r) ? ATTRIB_OPS[r.type] : CORE_OPS[r.field];
    },

    valueKind(r) {
      if (NO_VALUE_OPS.includes(r.op)) {
        return 'none';
      }
      if (r.op === 'in_last_days' || r.op === 'not_in_last_days') {
        return 'days';
      }
      if (r.op === 'before' || r.op === 'after') {
        return 'date';
      }
      if (r.field === 'list' || r.field === 'campaign') {
        return r.field;
      }
      if (r.field === 'status' && r.op !== 'one_of') {
        return 'status';
      }
      return r.type === 'number' ? 'number' : 'text';
    },

    detectedType(field) {
      const a = this.attribKeys.find((k) => ATTRIB + k.key === field);
      return a ? a.type : 'text';
    },

    defaultValue(r) {
      return r.field === 'status' ? 'enabled' : '';
    },

    addRule(field, op) {
      nextID += 1;
      const r = {
        id: nextID, field, type: '', op, value: '',
      };
      r.value = this.defaultValue(r);
      this.rules.push(r);
      this.emit();
    },

    removeRule(r) {
      this.rules.splice(this.rules.indexOf(r), 1);
      this.emit();
    },

    onFieldChange(r, field) {
      const next = { ...r, field, type: field.startsWith(ATTRIB) ? this.detectedType(field) : '' };
      [next.op] = this.opsFor(next);
      next.value = this.defaultValue(next);
      this.rules.splice(this.rules.indexOf(r), 1, next);
      this.emit();
    },

    toSegment() {
      if (this.rules.length === 0) {
        return null;
      }

      return {
        match: this.match,
        rules: this.rules.map((r) => ({
          field: r.field,
          ...(this.isAttrib(r) ? { type: r.type } : {}),
          op: r.op,
          value: r.value === '' || NO_VALUE_OPS.includes(r.op) ? null : r.value,
        })),
      };
    },

    fromRule(r) {
      nextID += 1;
      const legacyType = r.op === 'gt' || r.op === 'lt' ? 'number' : 'text';
      return {
        id: nextID,
        field: r.field,
        type: r.field.startsWith(ATTRIB) ? r.type || legacyType : '',
        op: r.op,
        value: r.value === null || r.value === undefined ? '' : r.value,
      };
    },

    fromSegment(seg) {
      this.match = (seg && seg.match) || 'all';
      this.rules = ((seg && seg.rules) || []).map(this.fromRule);
    },

    emit() {
      const seg = this.toSegment();
      this.lastEmitted = JSON.stringify(seg);
      this.$emit('input', seg);
    },
  },

  watch: {
    value: {
      immediate: true,
      handler(v) {
        if (JSON.stringify(v) !== this.lastEmitted) {
          this.fromSegment(v);
          this.lastEmitted = JSON.stringify(v);
        }
      },
    },
  },
});
</script>
