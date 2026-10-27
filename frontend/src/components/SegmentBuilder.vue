<template>
  <div class="segment-builder">
    <div v-if="rules.length > 1" class="match is-size-7 mb-2">
      {{ $t('campaigns.segmentMatch') }}
      <b-select v-model="match" size="is-small" :disabled="disabled" @input="emit">
        <option value="all">{{ $t('campaigns.segmentMatchAll') }}</option>
        <option value="any">{{ $t('campaigns.segmentMatchAny') }}</option>
      </b-select>
    </div>

    <div v-for="(r, i) in rules" :key="r.id" class="rule columns is-variable is-1 is-multiline mb-0"
      data-cy="segment-rule">
      <div class="column is-narrow">
        <b-select v-model="r.field" size="is-small" :disabled="disabled" @input="onFieldChange(i)"
          data-cy="segment-field">
          <option v-for="f in fields" :key="f" :value="f">{{ $t(`campaigns.segmentField.${f}`) }}</option>
        </b-select>
      </div>
      <div v-if="r.field === 'attribs'" class="column is-narrow">
        <b-autocomplete v-model="r.key" size="is-small" :disabled="disabled" :data="attribKeysFor(r.key)"
          open-on-focus :placeholder="$t('campaigns.segmentAttribKey')" @input="emit" data-cy="segment-key" />
      </div>
      <div class="column is-narrow">
        <b-select v-model="r.op" size="is-small" :disabled="disabled" @input="emit" data-cy="segment-op">
          <option v-for="op in opsFor(r.field)" :key="op" :value="op">{{ $t(`campaigns.segmentOp.${op}`) }}</option>
        </b-select>
      </div>
      <div class="column">
        <b-select v-if="r.field === 'list'" v-model="r.value" expanded size="is-small" :disabled="disabled"
          @input="emit" data-cy="segment-value">
          <option v-for="l in lists" :key="l.id" :value="l.id">{{ l.name }}</option>
        </b-select>
        <b-select v-else-if="r.field === 'campaign'" v-model="r.value" expanded size="is-small" :disabled="disabled"
          @input="emit" data-cy="segment-value">
          <option v-for="c in campaigns" :key="c.id" :value="c.id">{{ c.name }}</option>
        </b-select>
        <b-select v-else-if="r.field === 'status' && r.op !== 'one_of'" v-model="r.value" expanded size="is-small"
          :disabled="disabled" @input="emit" data-cy="segment-value">
          <option v-for="s in statuses" :key="s" :value="s">{{ $t(`subscribers.status.${s}`) }}</option>
        </b-select>
        <span v-else-if="r.op === 'is_set' || r.op === 'not_set'" />
        <b-input v-else-if="isNumeric(r)" v-model.number="r.value" type="number" size="is-small" :disabled="disabled"
          @input="emit" data-cy="segment-value" />
        <b-input v-else-if="isDate(r)" v-model="r.value" type="date" size="is-small" :disabled="disabled"
          @input="emit" data-cy="segment-value" />
        <b-input v-else v-model="r.value" size="is-small" :disabled="disabled" @input="emit"
          :placeholder="r.op === 'one_of' ? 'a, b, c' : ''" data-cy="segment-value" />
      </div>
      <div class="column is-narrow">
        <b-button size="is-small" icon-left="trash-can-outline" :disabled="disabled" @click="removeRule(i)"
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

const OPS = {
  email: ['eq', 'neq', 'contains', 'not_contains', 'one_of'],
  name: ['eq', 'neq', 'contains', 'not_contains', 'one_of'],
  status: ['eq', 'neq', 'one_of'],
  created_at: ['after', 'before', 'in_last_days'],
  attribs: ['eq', 'neq', 'contains', 'not_contains', 'one_of', 'gt', 'lt', 'is_set', 'not_set'],
  list: ['in', 'not_in'],
  campaign: ['received', 'not_received', 'opened', 'not_opened'],
};

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
      fields: Object.keys(OPS),
      statuses: ['enabled', 'disabled', 'blocklisted'],
      match: 'all',
      rules: [],
      lastEmitted: null,
    };
  },

  methods: {
    opsFor(field) {
      return OPS[field] || [];
    },

    attribKeysFor(typed) {
      const q = (typed || '').toLowerCase();
      return this.attribKeys.filter((k) => k.toLowerCase().includes(q));
    },

    isNumeric(r) {
      return r.op === 'gt' || r.op === 'lt' || r.op === 'in_last_days';
    },

    isDate(r) {
      return r.field === 'created_at' && r.op !== 'in_last_days';
    },

    addRule(field, op) {
      nextID += 1;
      this.rules.push({
        id: nextID, field, key: '', op, value: field === 'status' ? 'enabled' : '',
      });
      this.emit();
    },

    removeRule(i) {
      this.rules.splice(i, 1);
      this.emit();
    },

    onFieldChange(i) {
      const r = this.rules[i];
      const [op] = this.opsFor(r.field);
      r.op = op;
      r.value = r.field === 'status' ? 'enabled' : '';
      this.emit();
    },

    toSegment() {
      if (this.rules.length === 0) {
        return null;
      }

      return {
        match: this.match,
        rules: this.rules.map((r) => ({
          field: r.field === 'attribs' ? `attribs.${r.key}` : r.field,
          op: r.op,
          value: r.value === '' ? null : r.value,
        })),
      };
    },

    fromSegment(seg) {
      this.match = (seg && seg.match) || 'all';
      this.rules = ((seg && seg.rules) || []).map((r) => {
        nextID += 1;
        const isAttrib = r.field.startsWith('attribs.');
        return {
          id: nextID,
          field: isAttrib ? 'attribs' : r.field,
          key: isAttrib ? r.field.slice('attribs.'.length) : '',
          op: r.op,
          value: r.value === null || r.value === undefined ? '' : r.value,
        };
      });
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
