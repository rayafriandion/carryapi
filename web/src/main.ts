import { createApp, h } from 'vue'
import { NMessageProvider, NCard, NConfigProvider } from 'naive-ui'

const App = {
  render() {
    return h(NConfigProvider, null, {
      default: () => h(NMessageProvider, null, {
        default: () => h(NCard, { title: 'carryAPI' }, {
          default: () => 'carryAPI management console - coming soon'
        })
      })
    })
  }
}

createApp(App).mount('#app')
