
<script setup lang="ts">import EntityPage from '../components/EntityPage.vue'; import { ENTITY_CONFIGS } from '../types/status'; import { usePriorityDecisionStore } from '../stores/priority-decision'; const store = usePriorityDecisionStore();</script>
<template><EntityPage :config="ENTITY_CONFIGS[3]" :store="store"/></template>
