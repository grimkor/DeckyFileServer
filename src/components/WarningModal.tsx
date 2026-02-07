import {
  DialogBody,
  DialogButton,
  DialogFooter,
  DialogHeader,
  ModalRoot,
} from "@decky/ui";

export const WarningModal = ({closeModal, onCancel, onConfirm}: {
  closeModal?: () => void;
  onCancel: () => void;
  onConfirm: () => Promise<void>;
}) => {

  const handleCancel = () => {
    onCancel();
    closeModal?.();
  }

  const handleConfirm = async () => {
    await onConfirm();
    closeModal?.();
  }

  return (
    <ModalRoot closeModal={handleCancel}>
      <DialogHeader>Warning</DialogHeader>
      <DialogBody>
        <p>
          Do not run this on an untrusted network as this will expose parts of the Steam Deck's file system to the
          network.
        </p>
        <p>
          When accessing the URL you will receive a certificate security warning. This is because the plugin is using a
          self-signed certificate.
        </p>
      </DialogBody>
      <DialogFooter>
        <DialogButton onClick={handleConfirm}>
          Got it!
        </DialogButton>
      </DialogFooter>
    </ModalRoot>
  );
}
