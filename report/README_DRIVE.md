# Google Drive Integration Guide

This guide details how to configure the Google Drive integration to automatically upload test reports (`.docx`) to the cloud Google Drive `report` folder.

## Setup Requirements

### 1. Enable Google Drive API & Get Credentials
1. Go to the [Google Cloud Console](https://console.cloud.google.com/).
2. Create a new project or select an existing one.
3. Enable the **Google Drive API**.
4. Go to **APIs & Services > Credentials**.
5. Click **Create Credentials > Service Account**.
6. Follow the steps, and once created, go to the Service Account details, select the **Keys** tab, and click **Add Key > Create New Key (JSON)**.
7. Save the downloaded JSON file as `service_account.json` in the root folder of this project (`f:\soulchip\service_account.json`).

### 2. Share Google Drive Folder (Optional)
If you want to save reports into a specific folder in a shared drive, share that folder with the `client_email` listed inside your `service_account.json` file.

### 3. Install Dependencies
Run the following command to install the required Python libraries:
```bash
pip install google-api-python-client google-auth-httplib2 google-auth-oauthlib
```

## Running the Upload Script
You can upload any file (like `.docx` reports) using the script:
```bash
python report/upload_to_drive.py report/test_report.docx
```
The script will:
1. Load credentials from `service_account.json`.
2. Check if a folder named `report` exists in Google Drive. If not, it will create it.
3. Upload the `.docx` file into that folder.
