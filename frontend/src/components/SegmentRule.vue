<template>
  <div class="rule columns is-variable is-1 mb-0" data-cy="segment-rule">
    <div class="column is-narrow">
      <b-select :value="value.field" size="is-small" :disabled="disabled" @input="onFieldChange"
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
      <b-select :value="value.op" size="is-small" :disabled="disabled" @input="(op) => update({ op })"
        data-cy="segment-op">
        <option v-for="op in ops" :key="op" :value="op">{{ $t(`campaigns.segmentOp.${op}`) }}</option>
      </b-select>
    </div>
    <div class="column">
      <template v-if="valueKind === 'none'" />
      <b-select v-else-if="valueKind === 'list'" :value="value.value" expanded size="is-small" :disabled="disabled"
        @input="setValue" data-cy="segment-value">
        <option v-for="l in lists" :key="l.id" :value="l.id">{{ l.name }}</option>
      </b-select>
      <b-select v-else-if="valueKind === 'campaign'" :value="value.value" expanded size="is-small"
        :disabled="disabled" @input="setValue" data-cy="segment-value">
        <option v-for="c in campaigns" :key="c.id" :value="c.id">{{ c.name }}</option>
      </b-select>
      <b-select v-else-if="valueKind === 'status'" :value="value.value" expanded size="is-small" :disabled="disabled"
        @input="setValue" data-cy="segment-value">
        <option v-for="s in statuses" :key="s" :value="s">{{ $t(`subscribers.status.${s}`) }}</option>
      </b-select>
      <b-field v-else-if="valueKind === 'days'" class="days">
        <b-input :value="value.value" type="number" min="0" size="is-small" :disabled="disabled"
          @input="setNumber" data-cy="segment-value" />
        <p class="control">
          <span class="button is-static is-small">
            {{ value.op === 'not_in_last_days' ? $t('campaigns.segmentDaysAgo') : $t('campaigns.segmentDays') }}
          </span>
        </p>
      </b-field>
      <b-input v-else-if="valueKind === 'number'" :value="value.value" type="number" step="any" size="is-small"
        :disabled="disabled" @input="setNumber" data-cy="segment-value" />
      <b-input v-else-if="valueKind === 'date'" :value="value.value" type="date" size="is-small" :disabled="disabled"
        @input="setValue" data-cy="segment-value" />
      <b-input v-else :value="value.value" size="is-small" :disabled="disabled" @input="setValue"
        :placeholder="value.op === 'one_of' ? 'a, b, c' : ''" data-cy="segment-value" />
    </div>
    <div class="column is-narrow">
      <b-button size="is-small" icon-left="trash-can-outline" :disabled="disabled" @click="$emit('remove')"
        data-cy="segment-remove" />
    </div>
  </div>
</template>

<script>
import Vue from 'vue';
import {
  ATTRIB, ATTRIB_OPS, CORE_OPS, NO_VALUE_OPS,
} from '../segment';

export default Vue.extend({
  name: 'SegmentRule',

  props: {
    value: { type: Object, required: true },
    lists: { type: Array, default: () => [] },
    campaigns: { type: Array, default: () => [] },
    attribKeys: { type: Array, default: () => [] },
    disabled: Boolean,
  },

  data() {
    return {
      coreFields: Object.keys(CORE_OPS),
      statuses: ['enabled', 'disabled', 'blocklisted'],
    };
  },

  computed: {
    attribFields() {
      const fields = this.attribKeys.map((a) => ({ field: ATTRIB + a.key, key: a.key }));
      if (this.value.field.startsWith(ATTRIB) && !fields.some((a) => a.field === this.value.field)) {
        fields.push({ field: this.value.field, key: this.value.field.slice(ATTRIB.length) });
      }
      return fields;
    },

    ops() {
      return this.opsFor(this.value.field, this.value.type);
    },

    valueKind() {
      const { field, op, type } = this.value;
      if (NO_VALUE_OPS.includes(op)) {
        return 'none';
      }
      if (op === 'in_last_days' || op === 'not_in_last_days') {
        return 'days';
      }
      if (op === 'before' || op === 'after') {
        return 'date';
      }
      if (field === 'list' || field === 'campaign') {
        return field;
      }
      if (field === 'status' && op !== 'one_of') {
        return 'status';
      }
      return type === 'number' ? 'number' : 'text';
    },
  },

  methods: {
    opsFor(field, type) {
      return field.startsWith(ATTRIB) ? ATTRIB_OPS[type] : CORE_OPS[field];
    },

    update(patch) {
      this.$emit('input', { ...this.value, ...patch });
    },

    setValue(value) {
      this.update({ value });
    },

    setNumber(value) {
      this.update({ value: value === '' ? '' : Number(value) });
    },

    onFieldChange(field) {
      let type = '';
      if (field.startsWith(ATTRIB)) {
        const a = this.attribKeys.find((k) => ATTRIB + k.key === field);
        type = a ? a.type : 'text';
      }

      this.update({
        field, type, op: this.opsFor(field, type)[0], value: field === 'status' ? 'enabled' : '',
      });
    },
  },
});
</script>
