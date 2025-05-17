    const express = require('express');
    const axios = require('axios');
    const helper = require('./lib/helper');
    
    console.log('Starting main application...');
    console.log(helper.getMessage());
    
    // Show that we can access our dependencies
    console.log('Express version:', express.version);
    console.log('Axios version:', axios.version);
    
    // Some async operation
    setTimeout(() => {
        console.log('Async operation completed');
    }, 2000);

