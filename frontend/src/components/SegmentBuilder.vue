<template>
  <div class="segment-builder">
    <div v-if="items.length > 1" class="match is-size-7 mb-2">
      {{ $t('campaigns.segmentMatch') }}
      <b-select v-model="match" size="is-small" :disabled="disabled" @input="emit" data-cy="segment-match">
        <option value="all">{{ $t('campaigns.segmentMatchAll') }}</option>
        <option value="any">{{ $t('campaigns.segmentMatchAny') }}</option>
      </b-select>
    </div>

    <template v-for="item in items">
      <div v-if="item.rules" :key="item.id" class="group mb-3" data-cy="segment-group">
        <div class="is-flex is-justify-content-space-between is-align-items-center mb-2">
          <span class="match is-size-7">
            {{ $t('campaigns.segmentMatch') }}
            <b-select v-model="item.match" size="is-small" :disabled="disabled" @input="emit"
              data-cy="segment-group-match">
              <option value="all">{{ $t('campaigns.segmentMatchAll') }}</option>
              <option value="any">{{ $t('campaigns.segmentMatchAny') }}</option>
            </b-select>
          </span>
          <b-button size="is-small" icon-left="trash-can-outline" :disabled="disabled" @click="remove(items, item)"
            data-cy="segment-group-remove" />
        </div>

        <segment-rule v-for="r in item.rules" :key="r.id" :value="r" v-bind="ruleProps"
          @input="(next) => replace(item.rules, r, next)" @remove="removeFromGroup(item, r)" />

        <b-button size="is-small" icon-left="plus" :disabled="disabled" @click="addToGroup(item)"
          data-cy="segment-group-add">
          {{ $t('campaigns.segmentAddRule') }}
        </b-button>
      </div>

      <segment-rule v-else :key="item.id" :value="item" v-bind="ruleProps"
        @input="(next) => replace(items, item, next)" @remove="remove(items, item)" />
    </template>

    <p v-if="items.length === 0" class="is-size-7 has-text-grey mb-2">
      {{ $t('campaigns.segmentNoRules') }}
    </p>

    <div class="buttons are-small">
      <b-button size="is-small" icon-left="plus" :disabled="disabled" @click="addRule('email', 'contains')"
        data-cy="segment-add">
        {{ $t('campaigns.segmentAddRule') }}
      </b-button>
      <b-button size="is-small" icon-left="plus" :disabled="disabled" @click="addGroup" data-cy="segment-add-group">
        {{ $t('campaigns.segmentAddGroup') }}
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
import SegmentRule from './SegmentRule.vue';
import { ATTRIB, NO_VALUE_OPS } from '../segment';

let nextID = 1;

const newID = () => {
  nextID += 1;
  return nextID;
};

export default Vue.extend({
  name: 'SegmentBuilder',

  components: { SegmentRule },

  props: {
    value: { type: Object, default: null },
    lists: { type: Array, default: () => [] },
    campaigns: { type: Array, default: () => [] },
    attribKeys: { type: Array, default: () => [] },
    disabled: Boolean,
  },

  data() {
    return {
      match: 'all',
      items: [],
      lastEmitted: null,
    };
  },

  computed: {
    ruleProps() {
      return {
        lists: this.lists, campaigns: this.campaigns, attribKeys: this.attribKeys, disabled: this.disabled,
      };
    },
  },

  methods: {
    newRule(field, op) {
      return {
        id: newID(), field, type: '', op, value: field === 'status' ? 'enabled' : '',
      };
    },

    addRule(field, op) {
      this.items.push(this.newRule(field, op));
      this.emit();
    },

    addGroup() {
      this.items.push({ id: newID(), match: 'all', rules: [this.newRule('email', 'contains')] });
      this.emit();
    },

    addToGroup(group) {
      group.rules.push(this.newRule('email', 'contains'));
      this.emit();
    },

    replace(list, old, next) {
      list.splice(list.indexOf(old), 1, next);
      this.emit();
    },

    remove(list, item) {
      list.splice(list.indexOf(item), 1);
      this.emit();
    },

    removeFromGroup(group, r) {
      if (group.rules.length === 1) {
        this.remove(this.items, group);
        return;
      }
      this.remove(group.rules, r);
    },

    ruleOut(r) {
      return {
        field: r.field,
        ...(r.field.startsWith(ATTRIB) ? { type: r.type } : {}),
        op: r.op,
        value: r.value === '' || NO_VALUE_OPS.includes(r.op) ? null : r.value,
      };
    },

    ruleIn(r) {
      const legacyType = r.op === 'gt' || r.op === 'lt' ? 'number' : 'text';
      return {
        id: newID(),
        field: r.field,
        type: r.field.startsWith(ATTRIB) ? r.type || legacyType : '',
        op: r.op,
        value: r.value === null || r.value === undefined ? '' : r.value,
      };
    },

    toSegment() {
      if (this.items.length === 0) {
        return null;
      }

      return {
        match: this.match,
        rules: this.items.map((i) => (i.rules ? { match: i.match, rules: i.rules.map(this.ruleOut) } : this.ruleOut(i))),
      };
    },

    fromSegment(seg) {
      this.match = (seg && seg.match) || 'all';
      this.items = ((seg && seg.rules) || []).map((i) => (i.rules
        ? { id: newID(), match: i.match || 'all', rules: i.rules.map(this.ruleIn) }
        : this.ruleIn(i)));
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
