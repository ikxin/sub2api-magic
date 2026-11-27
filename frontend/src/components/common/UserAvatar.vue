<template>
  <img
    :key="imageSrc"
    :src="imageSrc"
    :alt="alt"
    class="shrink-0 rounded-[22%] bg-primary-100 object-cover dark:bg-primary-900/30"
    decoding="async"
    @error="handleImageError"
  >
</template>

<script lang="ts">
import { Avatar, Style, type StyleDefinition } from '@dicebear/core/lite'
import clay from '@dicebear/styles/clay.json'

const clayStyle = new Style(clay as StyleDefinition)
</script>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'

const props = withDefaults(defineProps<{
  email?: string | null
  src?: string | null
  alt?: string
}>(), {
  email: null,
  src: null,
  alt: '',
})

const customSrc = computed(() => props.src?.trim() || '')
const customImageFailed = ref(false)
watch(customSrc, () => {
  customImageFailed.value = false
})

const defaultSrc = computed(() => new Avatar(clayStyle, {
  seed: props.email?.trim().toLowerCase() || 'default',
}).toDataUri())

const imageSrc = computed(() => customSrc.value && !customImageFailed.value
  ? customSrc.value
  : defaultSrc.value)

function handleImageError(event: Event) {
  const image = event.currentTarget as HTMLImageElement
  if (customSrc.value && image.getAttribute('src') === customSrc.value) {
    customImageFailed.value = true
  }
}
</script>
