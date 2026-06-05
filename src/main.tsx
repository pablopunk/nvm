import React from 'react'
import ReactDOM from 'react-dom/client'
import { App, ExtensionWindowApp } from './App'
import './styles.css'

const extensionWindowId = new URLSearchParams(window.location.search).get('extensionWindowId')

ReactDOM.createRoot(document.getElementById('root')!).render(
  <React.StrictMode>
    {extensionWindowId ? <ExtensionWindowApp windowId={extensionWindowId} /> : <App />}
  </React.StrictMode>,
)
