export interface State {
  server_running: boolean;
  directory: string;
  port: number;
  timeout: number;
  allow_uploads: boolean;
  ip_address: string;
  error?: string;
  accepted_warning: boolean;
  history: string[];
  disable_thumbnails: boolean;
}
