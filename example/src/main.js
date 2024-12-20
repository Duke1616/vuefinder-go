import { createApp } from "vue";
import VueFinder from "vuefinder/dist/vuefinder";
import App from "./App.vue";
import "vuefinder/dist/style.css";

const app = createApp(App);
app.use(VueFinder, {
  locale: "zhCN",
  i18n: {
    en: async () => await import("vuefinder/dist/locales/en.js"),
    zhCN: async () => await import("vuefinder/dist/locales/zhCN.js"),
  },
});

app.mount("#app");
