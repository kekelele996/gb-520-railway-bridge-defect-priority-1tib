<script setup lang="ts">
import { computed } from 'vue';
const props = defineProps<{ severity: string }>();
const label = computed(() => ({ low: '低', medium: '中', high: '高', critical: '严重' }[props.severity] || props.severity));
</script>
<template><span :class="`severity severity--${severity}`">{{ label }}</span></template>
