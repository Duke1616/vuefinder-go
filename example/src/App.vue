<template>
  <div class="wrapper">
    <vue-finder
      id="vuefinder"
      :request="request"
      locale="zhCN"
      :full-screen="true"
      :max-file-size="maxFileSize"
      loadingIndicator="linear"
      :select-button="handleSelectButton"
    />
  </div>
</template>

<script setup>
import { ref } from "vue";

const request = {
  baseUrl: "http://127.0.0.1:8350/api/finder/index",
  params: { id: 20 },
  transformRequest: (req) => {
    switch (req.params.q) {
      case "upload":
        req.url = "http://127.0.0.1:8350/api/finder/upload";
        break;
      case "download":
        req.url = "http://127.0.0.1:8350/api/finder/download";
        break;
      case "rename":
        req.url = "http://127.0.0.1:8350/api/finder/rename";
        break;
      case "newfile":
        req.url = "http://127.0.0.1:8350/api/finder/new_file";
        break;
      case "newfolder":
        req.url = "http://127.0.0.1:8350/api/finder/new_folder";
        break;
      case "delete":
        req.url = "http://127.0.0.1:8350/api/finder/remove";
        break;
      case "subfolders":
        req.url = "http://127.0.0.1:8350/api/finder/subfolders";
        break;
      case "move":
        req.url = "http://127.0.0.1:8350/api/finder/move";
        break;
      case "archive":
        req.url = "http://127.0.0.1:8350/api/finder/archive";
        break;
      case "search":
        req.url = "http://127.0.0.1:8350/api/finder/search";
        break;
      case "preview":
        req.url = "http://127.0.0.1:8350/api/finder/preview";
        break;
      case "save":
        req.url = "http://127.0.0.1:8350/api/finder/save";
        break;
      default:
        break;
    }
    return req;
  },
};

const maxFileSize = ref("600MB");
const handleSelectButton = {
  active: true,
  multiple: false,
  click: (items, event) => {
    if (!items.length) {
      alert("No item selected");
      return;
    }
    alert("Selected: " + items[0].path);
    console.log(items, event);
  },
};
</script>

<style lang="scss">
body {
  margin: 0;
  background: #eeeeee;
}
.wrapper {
  max-width: 800px;
  margin: 80px auto;
}
.btn {
  display: block;
  margin: 20px auto;
  padding: 10px 20px;
  border: 1px solid #ccc;
  border-radius: 5px;
  background: #fff;
  cursor: pointer;
  outline: none;
}
</style>
