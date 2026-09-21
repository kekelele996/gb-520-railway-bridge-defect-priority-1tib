
<script setup lang="ts">import EntityPage from '../components/EntityPage.vue'; import { ENTITY_CONFIGS } from '../types/status'; import { useDefectFindingStore } from '../stores/defect-finding'; const store = useDefectFindingStore();</script>
<template><EntityPage :config="ENTITY_CONFIGS[2]" :store="store" show-evidence/></template>
