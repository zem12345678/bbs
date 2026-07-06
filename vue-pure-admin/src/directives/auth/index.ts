import { hasPerms } from "@/utils/auth";
import type { Directive, DirectiveBinding } from "vue";

export const auth: Directive = {
  mounted(el: HTMLElement, binding: DirectiveBinding<string | Array<string>>) {
    const { value } = binding;
    if (value) {
      !hasPerms(value) && el.parentNode?.removeChild(el);
    } else {
      throw new Error(
        "[Directive: auth]: need perms! Like v-auth=\"['btn.add','btn.edit']\""
      );
    }
  }
};
