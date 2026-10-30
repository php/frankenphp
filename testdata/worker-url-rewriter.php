<?php

require_once __DIR__.'/_executor.php';

return function () {
	if (isset($_GET['leak'])) {
		output_add_rewrite_var('token', "secret_{$_GET['leak']}");

		return;
	}

	echo json_encode(output_add_rewrite_var('x', 'y') && in_array('URL-Rewriter', ob_list_handlers(), true));
};
