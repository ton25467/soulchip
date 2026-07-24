import os
import sys

try:
    from google.oauth2 import service_account
    from googleapiclient.discovery import build
    from googleapiclient.http import MediaFileUpload
except ImportError:
    print("Error: Missing required Python packages. Please install them using:")
    print("pip install google-api-python-client google-auth-httplib2 google-auth-oauthlib")
    sys.exit(1)

# Path to the service account JSON key file in the project root
CREDENTIALS_FILE = 'service_account.json'

def get_drive_service():
    if not os.path.exists(CREDENTIALS_FILE):
        print(f"Error: Credentials file '{CREDENTIALS_FILE}' not found.")
        print(f"Please place your Google Cloud Service Account JSON key as '{CREDENTIALS_FILE}' in the project root.")
        sys.exit(1)
    
    scopes = ['https://www.googleapis.com/auth/drive']
    creds = service_account.Credentials.from_service_account_file(CREDENTIALS_FILE, scopes=scopes)
    return build('drive', 'v3', credentials=creds)

def get_or_create_folder(service, folder_name):
    # Search for folder
    query = f"name = '{folder_name}' and mimeType = 'application/vnd.google-apps.folder' and trashed = false"
    results = service.files().list(q=query, spaces='drive', fields='files(id, name)').execute()
    items = results.get('files', [])
    
    if items:
        print(f"Folder '{folder_name}' already exists (ID: {items[0]['id']})")
        return items[0]['id']
    
    # Create folder
    file_metadata = {
        'name': folder_name,
        'mimeType': 'application/vnd.google-apps.folder'
    }
    folder = service.files().create(body=file_metadata, fields='id').execute()
    print(f"Created folder '{folder_name}' (ID: {folder['id']})")
    return folder['id']

def upload_file(service, folder_id, file_path, mime_type):
    if not os.path.exists(file_path):
        print(f"Error: File '{file_path}' not found.")
        return
    
    file_name = os.path.basename(file_path)
    file_metadata = {
        'name': file_name,
        'parents': [folder_id]
    }
    
    media = MediaFileUpload(file_path, mimetype=mime_type, resumable=True)
    file = service.files().create(body=file_metadata, media_body=media, fields='id').execute()
    print(f"Uploaded file '{file_name}' successfully (ID: {file['id']})")

if __name__ == '__main__':
    if len(sys.argv) < 2:
        print("Usage: python upload_to_drive.py <path_to_docx_file>")
        sys.exit(1)
        
    file_to_upload = sys.argv[1]
    service = get_drive_service()
    folder_id = get_or_create_folder(service, 'report')
    upload_file(service, folder_id, file_to_upload, 'application/vnd.openxmlformats-officedocument.wordprocessingml.document')
