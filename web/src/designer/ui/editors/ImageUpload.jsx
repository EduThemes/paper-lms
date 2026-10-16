/* eslint-disable react/prop-types -- Paper JSX components don't declare propTypes */
/**
 * Uploads an image into the course's Files and hands back `/api/v1/files/<id>/download` to use as the src,
 * so the picture travels with the course (copies, exports) instead of living on another site.
 */
import { useRef, useState } from 'react';
import { Upload } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { uploadDesignImage } from '@/services/api_designer';
import { Spinner } from '../components/controls';
import { useDesigner } from '../state/context';
import { useDesignerT } from '../i18n';

const MAX_BYTES = 10 * 1024 * 1024;
const TYPES = ['image/jpeg', 'image/png', 'image/gif', 'image/webp'];

export function ImageUpload({ label, onUploaded }) {
  const { t } = useDesignerT();
  const { courseId, showToast } = useDesigner();
  const input = useRef(null);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState(null);

  async function onFile(e) {
    const file = e.target.files?.[0];
    e.target.value = '';
    if (!file) return;
    if (!TYPES.includes(file.type)) {
      setError(t('banner.notImage'));
      return;
    }
    if (file.size > MAX_BYTES) {
      setError(t('banner.uploadFailed'));
      return;
    }
    setError(null);
    setBusy(true);
    try {
      const { src } = await uploadDesignImage(courseId, file);
      onUploaded(src);
      showToast({ message: t('banner.uploaded'), variant: 'success' });
    } catch {
      setError(t('banner.uploadFailed'));
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className="space-y-1.5">
      <div className="flex items-center gap-2">
        <Button type="button" size="sm" variant="outline" disabled={busy} onClick={() => input.current?.click()}>
          <Upload aria-hidden="true" />
          {busy ? t('banner.uploading') : label}
        </Button>
        {busy && <Spinner label={t('banner.uploading')} />}
      </div>
      <input ref={input} type="file" accept={TYPES.join(',')} className="hidden" tabIndex={-1} aria-hidden="true" onChange={onFile} />
      {error && (
        <p role="alert" className="text-xs text-accent-danger">
          {error}
        </p>
      )}
    </div>
  );
}
