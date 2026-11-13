<template>
  <section class="issue-template-picker">
    <!-- Invite theme banner: full width above the guide, rounded with a soft shadow. -->
    <img v-if="themeImage" :src="themeImage" class="issue-template-picker__banner" alt="" />

    <!-- Guide block sits between the banner and the catalog card, both full width. -->
    <div v-if="guideTitle || guideDescription" class="issue-template-picker__guide">
      <h1 v-if="guideTitle" class="issue-template-picker__title">
        {{ guideTitle }}
      </h1>
      <p v-if="guideDescription" class="issue-template-picker__desc">
        {{ guideDescription }}
      </p>
    </div>

    <div class="issue-template-picker__card">
      <a-empty
        v-if="!templates.length"
        class="issue-template-picker__empty"
        :description="$t('issue_invite_no_template')"
      />
      <ul v-else class="issue-template-picker__list">
        <li
          v-for="item in templates"
          :key="item.template_key"
          class="issue-template-picker__item"
          @click="select(item)"
        >
          <span class="issue-template-picker__label">
            <WorkItemTypePill
              :type="item.type_key"
              :icon-text="item.type_icon_text || ''"
              :icon-color="item.type_icon_color || ''"
              size="large"
            />
            <strong class="issue-template-picker__name">{{ item.name }}</strong>
          </span>
          <img :src="rightArrowIcon" class="issue-template-picker__arrow" alt="" />
        </li>
      </ul>
    </div>
  </section>
</template>

<script setup>
import rightArrowIcon from '@/image/invite/right-arrow.svg'
import WorkItemTypePill from '@/components/common/WorkItemTypePill.vue'

const props = defineProps({
  templates: { type: Array, default: () => [] },
  guideTitle: { type: String, default: '' },
  guideDescription: { type: String, default: '' },
  themeImage: { type: String, default: '' },
})
const emit = defineEmits(['select'])

function select(item) {
  // Emit only the public template key; the page navigates to the dedicated form route.
  emit('select', item?.template_key)
}
</script>

<style scoped lang="less">
.issue-template-picker {
  padding: 24px 24px 32px;

  // Theme banner: keep the uploaded aspect ratio instead of cropping it.
  &__banner {
    display: block;
    width: 100%;
    height: auto;
    border-radius: 16px;
    box-shadow: 0 14px 32px rgba(25, 43, 76, 0.06);
  }

  &__guide {
    margin-top: 24px;
  }

  &__title {
    margin: 0;
    color: #172033;
    font-size: 16px;
    font-weight: 600;
    line-height: 24px;
  }

  &__desc {
    margin: 8px 0 0;
    color: #595959;
    font-size: 14px;
    line-height: 22px;
    white-space: pre-wrap;
  }

  &__card {
    margin-top: 16px;
    overflow: hidden;
    background: #fff;
    border-radius: 12px;
    box-shadow: 0 2px 24px rgba(0, 0, 0, 0.08);
  }

  &__empty {
    padding: 32px 0;
  }

  &__list {
    list-style: none;
    margin: 0;
    padding: 0;
  }

  &__item {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 12px;
    padding: 24px;
    cursor: pointer;
    transition: background-color 0.15s ease;

    & + & {
      border-top: 1px solid #f0f0f0;
    }

    &:hover {
      background: #f2f4f7;
    }

    &:active {
      background: #e9edf3;
    }
  }

  &__label {
    display: flex;
    align-items: center;
    gap: 12px;
    min-width: 0;
  }

  &__name {
    overflow: hidden;
    color: #262626;
    font-size: 16px;
    font-weight: 600;
    line-height: 24px;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  // Blue right-arrow asset (20*20) matching the public invite brand color.
  &__arrow {
    flex: 0 0 auto;
    display: block;
    width: 20px;
    height: 20px;
  }
}

// Narrow screens (the invite link is mostly opened on phones): tighten the gutters.
@media (max-width: 575px) {
  .issue-template-picker {
    padding: 16px 16px 24px;

    &__guide {
      margin-top: 16px;
    }

    &__item {
      padding: 16px;
    }
  }
}
</style>
