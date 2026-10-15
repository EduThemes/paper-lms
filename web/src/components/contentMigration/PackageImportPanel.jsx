import React, { useRef, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Upload, FileArchive, AlertCircle, CheckCircle } from 'lucide-react';
import { useImportPackage } from '../../services/api_content_migration';
import { PACKAGE_TYPES, buildPackageSettings, dateShiftProblem, defaultDateShift, summarizeCounts } from '../../lib/contentMigration';
import DateShiftFields from './DateShiftFields';

// PackageImportPanel uploads an IMS Common Cartridge / Canvas export
// (.imscc or .zip) with the import options: content types to leave out
// and date adjustment.
export default function PackageImportPanel({ courseId }) {
  const { t } = useTranslation();
  const fileRef = useRef(null);
  const [file, setFile] = useState(null);
  const [exclude, setExclude] = useState([]);
  const [dateShift, setDateShift] = useState(defaultDateShift);
  const [notice, setNotice] = useState(null);

  const upload = useImportPackage(courseId, {
    onSuccess: (res) => {
      const counts = {
        modules: res?.modules_created, pages: res?.pages_created, assignments: res?.assignments_created,
        quizzes: res?.quizzes_created, discussion_topics: res?.discussions_created, module_items: res?.module_items_created,
      };
      setNotice({ ok: true, text: t('contentImport.uploadSuccess', { name: file?.name }) + (summarizeCounts(counts) ? ` (${summarizeCounts(counts)})` : '') });
      setFile(null);
      if (fileRef.current) fileRef.current.value = '';
    },
    onError: (err) => setNotice({ ok: false, text: err.message || t('contentImport.uploadFailed') }),
  });

  const choose = (e) => {
    const f = e.target.files?.[0];
    setNotice(null);
    if (!f) return;
    const ext = f.name.toLowerCase().slice(f.name.lastIndexOf('.'));
    if (!['.imscc', '.zip'].includes(ext)) {
      setNotice({ ok: false, text: t('contentMigration.package.invalidType', 'Please select an .imscc or .zip package') });
      setFile(null);
      return;
    }
    setFile(f);
  };

  const dsProblem = dateShiftProblem(dateShift);
  const submit = (e) => {
    e.preventDefault();
    if (!file) return;
    setNotice(null);
    upload.mutate({ file, settings: buildPackageSettings({ excludeTypes: exclude, dateShift }) });
  };

  return (
    <form onSubmit={submit} className="space-y-5" aria-label={t('contentImport.uploadAPackage')}>
      <div className={`border-2 border-dashed rounded-lg p-6 text-center ${upload.isPending ? 'border-brand-300 bg-brand-50' : 'border-border-strong'}`}>
        <FileArchive className="w-10 h-10 text-text-tertiary mx-auto mb-2" />
        <p className="text-text-secondary font-medium mb-1">
          {file ? file.name : t('contentImport.uploadAPackage')}
        </p>
        <p className="text-text-tertiary text-xs mb-3">
          {t('contentMigration.package.formats', 'Canvas course export (.imscc) or Common Cartridge (.zip)')}
        </p>
        <label className="inline-flex items-center gap-2 px-4 py-2 rounded-md text-sm font-medium cursor-pointer bg-surface-2 text-text-primary hover:bg-surface-1">
          <Upload className="w-4 h-4" />
          {t('contentImport.chooseFile')}
          <input ref={fileRef} type="file" accept=".imscc,.zip" onChange={choose} className="sr-only" disabled={upload.isPending} />
        </label>
      </div>

      <fieldset>
        <legend className="text-sm font-medium text-text-primary mb-2">
          {t('contentMigration.package.include', 'Content to import')}
        </legend>
        <div className="grid grid-cols-2 sm:grid-cols-3 gap-2">
          {PACKAGE_TYPES.map((ty) => {
            const id = `pkg-${ty.key}`;
            return (
              <label key={ty.key} htmlFor={id} className="inline-flex items-center gap-2 text-sm text-text-secondary">
                <input
                  id={id}
                  type="checkbox"
                  checked={!exclude.includes(ty.key)}
                  onChange={(e) => setExclude(e.target.checked ? exclude.filter((k) => k !== ty.key) : [...exclude, ty.key])}
                />
                {ty.label}
              </label>
            );
          })}
        </div>
      </fieldset>

      <DateShiftFields
        value={dateShift}
        onChange={setDateShift}
        idPrefix="pkg-ds"
        oldDatesHint={t('contentMigration.package.dateHint', 'Give the original course start (and end) so dates can be moved; a package does not carry them.')}
      />

      {notice && (
        <div role={notice.ok ? 'status' : 'alert'} className={`p-3 rounded-md text-sm flex items-center gap-2 ${notice.ok ? 'bg-accent-success/10 text-accent-success' : 'bg-accent-danger/10 text-accent-danger'}`}>
          {notice.ok ? <CheckCircle className="w-4 h-4" /> : <AlertCircle className="w-4 h-4" />}
          {notice.text}
        </div>
      )}

      <button
        type="submit"
        disabled={!file || upload.isPending || !!dsProblem || exclude.length === PACKAGE_TYPES.length}
        className="inline-flex items-center gap-2 bg-brand-600 text-on-brand px-4 py-2 rounded-md text-sm font-medium hover:bg-brand-700 disabled:opacity-50 disabled:cursor-not-allowed"
      >
        <Upload className="w-4 h-4" />
        {upload.isPending ? t('contentImport.uploading') : t('contentMigration.package.submit', 'Import package')}
      </button>
    </form>
  );
}
