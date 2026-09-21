
<script setup lang="ts">import EntityPage from '../components/EntityPage.vue'; import { ENTITY_CONFIGS } from '../types/status'; import { useInspectionRoundStore } from '../stores/inspection-round'; const store = useInspectionRoundStore();</script>
<template><EntityPage :config="ENTITY_CONFIGS[1]" :store="store" show-evidence/></template>
