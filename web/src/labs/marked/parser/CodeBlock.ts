import { escape } from "lodash-es";
import hljs from "highlight.js";

export const CODE_BLOCK_REG = /^```(\S*?)\s([\s\S]*?)```(\n?)/;

const renderer = (rawStr: string): string => {
  const matchResult = rawStr.match(CODE_BLOCK_REG);
  if (!matchResult) {
    return rawStr;
  }

  const language = escape(matchResult[1]) || "plaintext";
  const highlightedCodes = hljs.highlight(matchResult[2], {
    language,
  }).value;

  return `<pre><code class="language-${language}">${highlightedCodes}</code></pre>${matchResult[3]}`;
};

export default {
  name: "code block",
  regex: CODE_BLOCK_REG,
  renderer,
};
