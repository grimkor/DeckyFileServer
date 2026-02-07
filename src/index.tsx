import { definePlugin, staticClasses } from "@decky/ui";
import { FaServer } from "react-icons/fa";
import { Content } from "./components/Content";

export default definePlugin(() => {
  return {
    title: <div className={staticClasses.Title}>Decky File Server</div>,
    content: <Content />,
    icon: <FaServer />,
    onDismount() {},
  };
});
