import React, { useState } from "react";

import Button from "components/buttons/Button";
import FileUploader from "components/FileUploader";
import InputField from "components/forms/fields/InputField";
import { IMofaAndroidApkFormData } from "services/entities/software";

const baseClass = "mofa-android-apk-form";

interface IMofaAndroidApkFormProps {
  isLoading: boolean;
  onCancel: () => void;
  onSubmit: (data: IMofaAndroidApkFormData) => void;
}

const MofaAndroidApkForm = ({
  isLoading,
  onCancel,
  onSubmit,
}: IMofaAndroidApkFormProps) => {
  const [software, setSoftware] = useState<File>();
  const [name, setName] = useState("");
  const [packageName, setPackageName] = useState("");
  const [versionName, setVersionName] = useState("");
  const [versionCode, setVersionCode] = useState("");
  const [hostIds, setHostIds] = useState("");

  const parsedHostIds = hostIds
    .split(",")
    .map((id) => Number(id.trim()))
    .filter((id) => Number.isInteger(id) && id > 0);
  const isValid =
    !!software &&
    name.trim() !== "" &&
    packageName.trim() !== "" &&
    versionName.trim() !== "" &&
    Number.isInteger(Number(versionCode)) &&
    Number(versionCode) > 0;

  return (
    <form
      className={baseClass}
      onSubmit={(event) => {
        event.preventDefault();
        if (!software || !isValid) return;
        onSubmit({
          software,
          name: name.trim(),
          packageName: packageName.trim(),
          versionName: versionName.trim(),
          versionCode: Number(versionCode),
          hostIds: parsedHostIds,
        });
      }}
    >
      <h2>Upload a company APK</h2>
      <p>
        Community workflow for Android devices without Google services. Android
        asks the user to approve each installation.
      </p>
      <FileUploader
        graphicName="file-pkg"
        accept=".apk,application/vnd.android.package-archive"
        message="APK files up to 200 MB"
        buttonMessage="Choose APK"
        onFileUpload={(files) => setSoftware(files?.[0])}
        fileDetails={software ? { name: software.name } : undefined}
        disabled={isLoading}
      />
      <div className={`${baseClass}__fields`}>
        <InputField
          label="App name"
          value={name}
          onChange={setName}
          disabled={isLoading}
        />
        <InputField
          label="Package name"
          placeholder="com.company.app"
          value={packageName}
          onChange={setPackageName}
          disabled={isLoading}
        />
        <InputField
          label="Version name"
          value={versionName}
          onChange={setVersionName}
          disabled={isLoading}
        />
        <InputField
          label="Version code"
          type="number"
          min={1}
          step={1}
          value={versionCode}
          onChange={setVersionCode}
          disabled={isLoading}
        />
        <InputField
          label="Android host IDs (optional)"
          helpText="Comma-separated. Leave empty to send to all Android hosts in this fleet."
          placeholder="12, 18, 25"
          value={hostIds}
          onChange={setHostIds}
          disabled={isLoading}
        />
      </div>
      <div className={`${baseClass}__actions`}>
        <Button type="submit" disabled={!isValid} isLoading={isLoading}>
          Upload and install
        </Button>
        <Button variant="secondary" onClick={onCancel} disabled={isLoading}>
          Cancel
        </Button>
      </div>
    </form>
  );
};

export default MofaAndroidApkForm;
