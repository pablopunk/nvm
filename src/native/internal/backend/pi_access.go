package backend

// PiAccess is what a process that talks to the backend's model proxy itself needs, such as the pi host: the origin, the
// bearer token of the signed-in user and the compatibility headers every request of this client carries. It fails when
// the user is not signed in.
func (c *Client) PiAccess() (baseURL, token string, headers map[string]string, err error) {
	token, err = c.tokens.Get()
	if err != nil {
		return "", "", nil, err
	}
	return c.env.BaseURL, token, c.identity.headerMap(), nil
}
