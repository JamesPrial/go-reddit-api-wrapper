<script lang="ts">
  import { realtimeData, ConnectionStatus } from '../stores/realtimeData';

  $: status = $realtimeData.status;
  $: error = $realtimeData.error;
  $: lastUpdate = $realtimeData.lastUpdate;

  function getStatusColor(status: ConnectionStatus): string {
    switch (status) {
      case ConnectionStatus.CONNECTED:
        return 'bg-green-500';
      case ConnectionStatus.CONNECTING:
      case ConnectionStatus.RECONNECTING:
        return 'bg-yellow-500';
      case ConnectionStatus.ERROR:
        return 'bg-red-500';
      case ConnectionStatus.DISCONNECTED:
      default:
        return 'bg-gray-500';
    }
  }

  function getStatusText(status: ConnectionStatus): string {
    switch (status) {
      case ConnectionStatus.CONNECTED:
        return 'Connected';
      case ConnectionStatus.CONNECTING:
        return 'Connecting...';
      case ConnectionStatus.RECONNECTING:
        return 'Reconnecting...';
      case ConnectionStatus.ERROR:
        return 'Error';
      case ConnectionStatus.DISCONNECTED:
      default:
        return 'Disconnected';
    }
  }

  function formatTime(date: Date | null): string {
    if (!date) return 'Never';
    return date.toLocaleTimeString();
  }
</script>

<div class="flex items-center gap-3 rounded-lg bg-gray-50 px-4 py-2 text-sm">
  <div class="flex items-center gap-2">
    <div class="relative flex h-3 w-3">
      <span
        class="absolute inline-flex h-full w-full animate-ping rounded-full opacity-75 {getStatusColor(status)}"
        class:hidden={status !== ConnectionStatus.CONNECTING && status !== ConnectionStatus.RECONNECTING}
      ></span>
      <span class="relative inline-flex h-3 w-3 rounded-full {getStatusColor(status)}"></span>
    </div>
    <span class="font-medium text-gray-700">{getStatusText(status)}</span>
  </div>

  {#if lastUpdate}
    <span class="text-gray-500">Last update: {formatTime(lastUpdate)}</span>
  {/if}

  {#if error}
    <span class="text-red-600" title={error}>Error</span>
  {/if}
</div>
