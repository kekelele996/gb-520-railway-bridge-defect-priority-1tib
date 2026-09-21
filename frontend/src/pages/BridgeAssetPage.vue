
<script setup lang="ts">import EntityPage from '../components/EntityPage.vue'; import { ENTITY_CONFIGS } from '../types/status'; import { useBridgeAssetStore } from '../stores/bridge-asset'; const store = useBridgeAssetStore();</script>
<template><EntityPage :config="ENTITY_CONFIGS[0]" :store="store"/></template>
