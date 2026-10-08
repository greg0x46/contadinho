import { App } from "antd";
import type { ReactNode } from "react";
import { useCallback } from "react";

export interface ConfirmOptions {
  title: string;
  description?: ReactNode;
  /** The action's verb, which is also the button ("Excluir", "Desvincular"). Defaults to "Excluir". */
  okText?: string;
  /** Red confirm button. Defaults to true: this is for what destroys or undoes something. */
  danger?: boolean;
  /**
   * What to do on confirm. Returning a promise keeps the button loading until
   * it settles, then the dialog closes; handle your own errors inside (an
   * inline Alert where the user is), a rejection would keep the dialog open.
   */
  onConfirm: () => void | Promise<unknown>;
}

/**
 * A confirmation that works from a menu item (a Popconfirm has no button to
 * anchor to there) and reads the same on a phone and a desktop: one antd
 * modal, "Cancelar" and the action's own verb. Deleting a record says
 * "Excluir"; "Remover" is only for unlinking. Outside an antd `<App>` — as in
 * most component tests — there is no modal API, so it falls back to the
 * browser's own confirm rather than crash.
 */
export function useConfirm(): (options: ConfirmOptions) => void {
  const { modal } = App.useApp();
  return useCallback(
    ({ title, description, okText = "Excluir", danger = true, onConfirm }) => {
      if (typeof modal?.confirm !== "function") {
        const text = typeof description === "string" ? `${title}\n${description}` : title;
        if (window.confirm(text)) void onConfirm();
        return;
      }
      modal.confirm({
        title,
        content: description,
        okText,
        cancelText: "Cancelar",
        okButtonProps: { danger },
        onOk: onConfirm,
      });
    },
    [modal],
  );
}
